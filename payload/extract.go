package payload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

type ExtractOptions struct {
	OutputDir   string
	Partitions  []string
	Concurrency int
	SourceDir   string
	SkipVerify  bool
	Logger      *slog.Logger
	OnProgress  ProgressFunc
}

func (p *Payload) Extract(ctx context.Context, opts ExtractOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = p.logger
	}
	selected, err := p.selectPartitions(opts.Partitions)
	if err != nil {
		return err
	}
	if err := checkSupported(selected); err != nil {
		return err
	}
	if err := checkSourceAvailable(selected, opts.SourceDir); err != nil {
		return err
	}
	if opts.OutputDir != "" {
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
			return fmt.Errorf("payload: creating output directory: %w", err)
		}
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	logger.Info("starting extraction",
		slog.Int("partitions", len(selected)),
		slog.Int("concurrency", concurrency),
		slog.Bool("delta", p.IsDelta()),
		slog.Uint64("minor_version", uint64(p.MinorVersion())),
	)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for _, part := range selected {
		part := part
		g.Go(func() error {
			return p.extractPartition(ctx, part, opts, logger)
		})
	}
	return g.Wait()
}

func (p *Payload) selectPartitions(names []string) ([]*chromeos_update_engine.PartitionUpdate, error) {
	all := p.manifest.GetPartitions()
	if len(names) == 0 {
		return all, nil
	}
	byName := make(map[string]*chromeos_update_engine.PartitionUpdate, len(all))
	for _, part := range all {
		byName[part.GetPartitionName()] = part
	}
	var selected []*chromeos_update_engine.PartitionUpdate
	var unknown []string
	for _, name := range names {
		part, ok := byName[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		selected = append(selected, part)
	}
	if len(unknown) > 0 {
		return nil, &UnknownPartitionsError{Names: unknown}
	}
	return selected, nil
}

func checkSupported(parts []*chromeos_update_engine.PartitionUpdate) error {
	unsupported := map[string][]string{}
	for _, part := range parts {
		if ops := partitionUnsupportedOps(part); len(ops) > 0 {
			unsupported[part.GetPartitionName()] = ops
		}
	}
	if len(unsupported) > 0 {
		return &UnsupportedOperationsError{Partitions: unsupported}
	}
	return nil
}

func checkSourceAvailable(parts []*chromeos_update_engine.PartitionUpdate, sourceDir string) error {
	if sourceDir != "" {
		return nil
	}
	var missing []string
	for _, part := range parts {
		if partitionNeedsSource(part) {
			missing = append(missing, part.GetPartitionName())
		}
	}
	if len(missing) > 0 {
		return &MissingSourceError{Partitions: missing}
	}
	return nil
}

func (p *Payload) extractPartition(ctx context.Context, part *chromeos_update_engine.PartitionUpdate, opts ExtractOptions, logger *slog.Logger) (err error) {
	name := part.GetPartitionName()
	totalOps := len(part.GetOperations())
	report := func(completed int, done bool, failure error) {
		if opts.OnProgress != nil {
			opts.OnProgress(ProgressEvent{
				Partition:    name,
				TotalOps:     totalOps,
				CompletedOps: completed,
				Done:         done,
				Err:          failure,
			})
		}
	}
	started := time.Now()
	logger.Info("extracting partition",
		slog.String("partition", name),
		slog.Int("operations", totalOps),
		slog.Bool("needs_source", partitionNeedsSource(part)),
	)
	report(0, false, nil)
	outPath := filepath.Join(opts.OutputDir, name+".img")
	completed := 0
	defer func() {
		if err != nil {
			report(completed, true, err)
			if removeErr := os.Remove(outPath); removeErr == nil {
				logger.Warn("removed incomplete output", slog.String("partition", name), slog.String("path", outPath))
			}
		}
	}()

	var src *sourceImage
	if partitionNeedsSource(part) {
		if err = checkFecReconstructible(part, p.blockSize); err != nil {
			return err
		}
		src, err = openSourceImage(opts.SourceDir, part, opts.SkipVerify, logger)
		if err != nil {
			return err
		}
		defer src.Close()
	}

	out, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("payload: creating output file: %w", err)
	}
	defer out.Close()
	newInfo := part.GetNewPartitionInfo()
	if size := newInfo.GetSize(); size > 0 {
		if err = out.Truncate(int64(size)); err != nil {
			return fmt.Errorf("payload: sizing output file to %d bytes: %w", size, err)
		}
	}

	if src != nil {
		if err = copySourceFallthrough(out, src, part, p.blockSize, logger); err != nil {
			return err
		}
	}

	x := &partitionExtractor{p: p, name: name, out: out, src: src, skipVerify: opts.SkipVerify}
	for i, op := range part.GetOperations() {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = x.runOp(i, op); err != nil {
			return err
		}
		completed = i + 1
		report(completed, false, nil)
	}

	if src != nil {
		if err = writeHashTree(out, part, p.blockSize, logger); err != nil {
			return err
		}
	}

	if err = p.verifyOutput(out, part, opts.SkipVerify, logger); err != nil {
		return err
	}
	logger.Info("partition extracted",
		slog.String("partition", name),
		slog.String("path", outPath),
		slog.Uint64("bytes", newInfo.GetSize()),
		slog.Duration("took", time.Since(started)),
	)
	report(totalOps, true, nil)
	return nil
}

func copySourceFallthrough(out *os.File, src *sourceImage, part *chromeos_update_engine.PartitionUpdate, blockSize uint64, logger *slog.Logger) error {
	newSize := int64(part.GetNewPartitionInfo().GetSize())
	if newSize <= 0 {
		return nil
	}
	totalBlocks := (uint64(newSize) + blockSize - 1) / blockSize
	var dst []*chromeos_update_engine.Extent
	for _, op := range part.GetOperations() {
		dst = append(dst, op.GetDstExtents()...)
	}
	gaps := uncoveredRanges(dst, totalBlocks)
	if len(gaps) == 0 {
		return nil
	}
	var copied int64
	for _, gap := range gaps {
		offset := int64(gap.start * blockSize)
		end := int64(gap.end * blockSize)
		if end > newSize {
			end = newSize
		}
		if offset >= src.size {
			continue
		}
		if end > src.size {
			end = src.size
		}
		n, err := io.Copy(io.NewOffsetWriter(out, offset), io.NewSectionReader(src.f, offset, end-offset))
		copied += n
		if err != nil {
			return fmt.Errorf("payload: copying unmodified blocks from source image: %w", err)
		}
	}
	logger.Debug("copied unmodified blocks from source image",
		slog.String("partition", part.GetPartitionName()),
		slog.Int("gap_ranges", len(gaps)),
		slog.Int64("bytes", copied),
	)
	return nil
}

func (p *Payload) verifyOutput(out *os.File, part *chromeos_update_engine.PartitionUpdate, skipVerify bool, logger *slog.Logger) error {
	newInfo := part.GetNewPartitionInfo()
	if skipVerify || len(newInfo.GetHash()) == 0 {
		return nil
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(out, 0, int64(newInfo.GetSize()))); err != nil {
		return fmt.Errorf("payload: hashing output: %w", err)
	}
	if sum := h.Sum(nil); !hashEqual(sum, newInfo.GetHash()) {
		return &VerificationError{
			Partition: part.GetPartitionName(),
			Subject:   "output image",
			Expected:  hex.EncodeToString(newInfo.GetHash()),
			Actual:    hex.EncodeToString(sum),
		}
	}
	logger.Debug("output hash verified", slog.String("partition", part.GetPartitionName()))
	return nil
}
