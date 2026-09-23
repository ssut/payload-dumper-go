package payload

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"github.com/ssut/payload-dumper-go/internal/fec"
)

func writeHashTree(ctx context.Context, out *os.File, part *chromeos_update_engine.PartitionUpdate, blockSize uint64, logger *slog.Logger) error {
	treeExtent := part.GetHashTreeExtent()
	dataExtent := part.GetHashTreeDataExtent()
	if treeExtent == nil || dataExtent == nil {
		return nil
	}
	algorithm := part.GetHashTreeAlgorithm()
	if algorithm == "" {
		algorithm = "sha1"
	}
	newHash, digestLen, err := hashTreeHasher(algorithm)
	if err != nil {
		return fmt.Errorf("payload: partition %q: %w", part.GetPartitionName(), err)
	}
	hashSize := nextPowerOfTwo(digestLen)
	if blockSize < uint64(hashSize)*2 || blockSize&(blockSize-1) != 0 {
		return fmt.Errorf("payload: partition %q: invalid hash tree block size %d", part.GetPartitionName(), blockSize)
	}
	if err := validateExtents([]*chromeos_update_engine.Extent{treeExtent, dataExtent}, blockSize); err != nil {
		return fmt.Errorf("payload: partition %q: %w", part.GetPartitionName(), err)
	}
	for _, extent := range []*chromeos_update_engine.Extent{treeExtent, dataExtent} {
		if (extent.GetStartBlock()+extent.GetNumBlocks())*blockSize > part.GetNewPartitionInfo().GetSize() {
			return fmt.Errorf("payload: partition %q: hash tree extent exceeds partition size", part.GetPartitionName())
		}
	}
	if extentsOverlap(treeExtent, dataExtent) {
		return fmt.Errorf("payload: partition %q: hash tree extent overlaps hash tree data extent", part.GetPartitionName())
	}
	var treeBlocks uint64
	for blocks := dataExtent.GetNumBlocks(); ; {
		blocks = (blocks + blockSize/uint64(hashSize) - 1) / (blockSize / uint64(hashSize))
		treeBlocks += blocks
		if blocks <= 1 {
			break
		}
	}
	if treeBlocks != treeExtent.GetNumBlocks() || treeBlocks > uint64(^uint(0)>>1)/blockSize {
		return fmt.Errorf("payload: partition %q: computed hash tree size does not match addressable hash_tree extent size", part.GetPartitionName())
	}
	salt := part.GetHashTreeSalt()
	dataOffset := int64(dataExtent.GetStartBlock() * blockSize)
	dataBlocks := dataExtent.GetNumBlocks()

	level := make([]byte, 0, (dataBlocks*uint64(hashSize)+blockSize-1)/blockSize*blockSize)
	buf := make([]byte, blockSize)
	digest := make([]byte, 0, digestLen)
	reader := io.NewSectionReader(out, dataOffset, int64(dataBlocks*blockSize))
	for i := uint64(0); i < dataBlocks; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := io.ReadFull(reader, buf); err != nil {
			return fmt.Errorf("payload: reading hash tree data block %d: %w", i, err)
		}
		h := newHash()
		h.Write(salt)
		h.Write(buf)
		digest = h.Sum(digest[:0])
		level = append(level, digest...)
		level = append(level, make([]byte, hashSize-digestLen)...)
	}
	level = padToBlock(level, blockSize)

	levels := [][]byte{level}
	for uint64(len(levels[len(levels)-1]))/blockSize > 1 {
		prev := levels[len(levels)-1]
		next := make([]byte, 0, (uint64(len(prev))/blockSize*uint64(hashSize)+blockSize-1)/blockSize*blockSize)
		for off := 0; off < len(prev); off += int(blockSize) {
			if err := ctx.Err(); err != nil {
				return err
			}
			h := newHash()
			h.Write(salt)
			h.Write(prev[off : off+int(blockSize)])
			digest = h.Sum(digest[:0])
			next = append(next, digest...)
			next = append(next, make([]byte, hashSize-digestLen)...)
		}
		next = padToBlock(next, blockSize)
		levels = append(levels, next)
	}

	var tree []byte
	for i := len(levels) - 1; i >= 0; i-- {
		tree = append(tree, levels[i]...)
	}
	expected := treeExtent.GetNumBlocks() * blockSize
	if uint64(len(tree)) != expected {
		return fmt.Errorf("payload: partition %q: computed hash tree size %d does not match hash_tree extent size %d", part.GetPartitionName(), len(tree), expected)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := out.WriteAt(tree, int64(treeExtent.GetStartBlock()*blockSize)); err != nil {
		return fmt.Errorf("payload: writing hash tree: %w", err)
	}
	logger.Debug("computed dm-verity hash tree",
		slog.String("partition", part.GetPartitionName()),
		slog.String("algorithm", algorithm),
		slog.Int("levels", len(levels)),
		slog.Int("bytes", len(tree)),
	)
	return nil
}

const (
	minFecRoots = 2
	maxFecRoots = 24
)

type verityOptions struct {
	sem         *semaphore.Weighted
	workers     int
	batchRounds int
	noFEC       bool
	progress    func(done, total int)
}

func writeVerity(ctx context.Context, out *os.File, part *chromeos_update_engine.PartitionUpdate, blockSize uint64, opts verityOptions, logger *slog.Logger) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	name := part.GetPartitionName()
	if part.GetHashTreeExtent().GetNumBlocks() > 0 {
		if part.GetHashTreeDataExtent().GetNumBlocks() == 0 {
			return false, fmt.Errorf("payload: partition %q: hash_tree_extent is set but hash_tree_data_extent is missing", name)
		}
		if err := writeHashTree(ctx, out, part, blockSize, logger); err != nil {
			return false, err
		}
	}
	if part.GetFecExtent().GetNumBlocks() == 0 {
		return false, nil
	}
	if opts.noFEC {
		logger.Warn("skipped FEC generation (-no-fec); image does not match its expected sha256 and must not be flashed",
			slog.String("partition", name))
		return true, nil
	}
	return false, writeFEC(ctx, out, part, blockSize, opts, logger)
}

func writeFEC(ctx context.Context, out *os.File, part *chromeos_update_engine.PartitionUpdate, blockSize uint64, opts verityOptions, logger *slog.Logger) error {
	name := part.GetPartitionName()
	fecExt := part.GetFecExtent()
	dataExt := part.GetFecDataExtent()
	treeExt := part.GetHashTreeExtent()

	if err := validateExtents([]*chromeos_update_engine.Extent{fecExt, dataExt, treeExt}, blockSize); err != nil {
		return fmt.Errorf("payload: partition %q: %w", name, err)
	}
	if dataExt.GetNumBlocks() == 0 {
		return fmt.Errorf("payload: partition %q: fec_extent is set but fec_data_extent is missing", name)
	}
	if blockSize != fec.BlockSize {
		return fmt.Errorf("payload: partition %q: FEC generation requires block size %d, got %d", name, fec.BlockSize, blockSize)
	}
	roots := int(part.GetFecRoots())
	if roots < minFecRoots || roots > maxFecRoots {
		return fmt.Errorf("payload: partition %q: fec_roots %d is outside the supported range %d..%d", name, roots, minFecRoots, maxFecRoots)
	}

	if dataExt.GetNumBlocks() > uint64(^uint(0)>>1) {
		return fmt.Errorf("payload: partition %q: fec data block count exceeds addressable range", name)
	}
	params := fec.Params{DataBlocks: int(dataExt.GetNumBlocks()), Roots: roots, BlockSize: blockSize, BatchRounds: opts.batchRounds}
	if want := uint64(params.Rounds()) * uint64(roots); fecExt.GetNumBlocks() != want {
		return fmt.Errorf("payload: partition %q: fec extent has %d blocks, expected %d (rounds=%d, fec_roots=%d)",
			name, fecExt.GetNumBlocks(), want, params.Rounds(), roots)
	}
	if extentsOverlap(fecExt, dataExt) {
		return fmt.Errorf("payload: partition %q: fec extent overlaps fec data extent", name)
	}
	if extentsOverlap(fecExt, treeExt) {
		return fmt.Errorf("payload: partition %q: fec extent overlaps hash tree extent", name)
	}
	size := part.GetNewPartitionInfo().GetSize()
	for _, e := range []struct {
		what string
		ext  *chromeos_update_engine.Extent
	}{{"fec data", dataExt}, {"fec", fecExt}} {
		if end := (e.ext.GetStartBlock() + e.ext.GetNumBlocks()) * blockSize; end > size {
			return fmt.Errorf("payload: partition %q: %s extent ends at %d, beyond partition size %d", name, e.what, end, size)
		}
	}

	started := time.Now()
	dataOffset := int64(dataExt.GetStartBlock() * blockSize)
	fecOffset := int64(fecExt.GetStartBlock() * blockSize)
	if err := fec.Generate(ctx, out, out, dataOffset, fecOffset, params, opts.workers, opts.sem, opts.progress); err != nil {
		return fmt.Errorf("payload: partition %q: %w", name, err)
	}
	logger.Debug("computed dm-verity fec",
		slog.String("partition", name),
		slog.Int("roots", roots),
		slog.Int("rounds", params.Rounds()),
		slog.Int("batches", params.Batches()),
		slog.Int64("bytes", params.ParityBytes()),
		slog.Duration("took", time.Since(started)),
	)
	return nil
}

func extentsOverlap(a, b *chromeos_update_engine.Extent) bool {
	if a.GetNumBlocks() == 0 || b.GetNumBlocks() == 0 {
		return false
	}
	aStart, aEnd := a.GetStartBlock(), a.GetStartBlock()+a.GetNumBlocks()
	bStart, bEnd := b.GetStartBlock(), b.GetStartBlock()+b.GetNumBlocks()
	return aStart < bEnd && bStart < aEnd
}

func hashTreeHasher(algorithm string) (func() hash.Hash, int, error) {
	switch algorithm {
	case "sha256":
		return sha256.New, sha256.Size, nil
	case "sha1":
		return sha1.New, sha1.Size, nil
	default:
		return nil, 0, fmt.Errorf("unsupported hash tree algorithm %q", algorithm)
	}
}

func nextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

func padToBlock(data []byte, blockSize uint64) []byte {
	if remainder := uint64(len(data)) % blockSize; remainder != 0 {
		data = append(data, make([]byte, blockSize-remainder)...)
	}
	return data
}
