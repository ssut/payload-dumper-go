package payload

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"google.golang.org/protobuf/proto"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

type Payload struct {
	header     Header
	manifest   *chromeos_update_engine.DeltaArchiveManifest
	ra         io.ReaderAt
	size       int64
	dataOffset int64
	blockSize  uint64
	closers    []io.Closer
	tempPath   string
	logger     *slog.Logger
}

func NewFromReaderAt(ra io.ReaderAt, size int64) (*Payload, error) {
	header, err := readHeader(io.NewSectionReader(ra, 0, size))
	if err != nil {
		return nil, err
	}
	metadataLen := int64(headerLen) + int64(header.ManifestLen) + int64(header.MetadataSignatureLen)
	if int64(header.ManifestLen) < 0 || metadataLen < headerLen || metadataLen > size {
		return nil, fmt.Errorf("payload: corrupt metadata sizes (manifest=%d signature=%d payload=%d)", header.ManifestLen, header.MetadataSignatureLen, size)
	}
	manifestBuf := make([]byte, header.ManifestLen)
	if _, err := io.ReadFull(io.NewSectionReader(ra, headerLen, int64(header.ManifestLen)), manifestBuf); err != nil {
		return nil, fmt.Errorf("payload: reading manifest: %w", err)
	}
	manifest := &chromeos_update_engine.DeltaArchiveManifest{}
	if err := proto.Unmarshal(manifestBuf, manifest); err != nil {
		return nil, fmt.Errorf("payload: parsing manifest: %w", err)
	}
	blockSize := uint64(manifest.GetBlockSize())
	if blockSize == 0 {
		blockSize = 4096
	}
	return &Payload{
		header:     header,
		manifest:   manifest,
		ra:         ra,
		size:       size,
		dataOffset: metadataLen,
		blockSize:  blockSize,
		logger:     discardLogger(),
	}, nil
}

func (p *Payload) Close() error {
	var firstErr error
	for i := len(p.closers) - 1; i >= 0; i-- {
		if err := p.closers[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.closers = nil
	if p.tempPath != "" {
		if err := os.Remove(p.tempPath); err != nil && firstErr == nil {
			firstErr = err
		}
		p.tempPath = ""
	}
	return firstErr
}

func (p *Payload) SetLogger(logger *slog.Logger) {
	if logger == nil {
		logger = discardLogger()
	}
	p.logger = logger
}

func (p *Payload) Header() Header {
	return p.header
}

func (p *Payload) Manifest() *chromeos_update_engine.DeltaArchiveManifest {
	return p.manifest
}

func (p *Payload) BlockSize() uint64 {
	return p.blockSize
}

func (p *Payload) MinorVersion() uint32 {
	return p.manifest.GetMinorVersion()
}

func (p *Payload) IsDelta() bool {
	if p.manifest.GetMinorVersion() != 0 || p.manifest.GetPartialUpdate() {
		return true
	}
	for _, part := range p.manifest.GetPartitions() {
		if partitionNeedsSource(part) {
			return true
		}
	}
	return false
}

type Partition struct {
	Name                  string
	Size                  uint64
	Hash                  []byte
	OldSize               uint64
	OldHash               []byte
	NeedsSource           bool
	TotalOperations       int
	UnsupportedOperations []string
}

func (p *Payload) Partitions() []Partition {
	parts := make([]Partition, 0, len(p.manifest.GetPartitions()))
	for _, part := range p.manifest.GetPartitions() {
		parts = append(parts, Partition{
			Name:                  part.GetPartitionName(),
			Size:                  part.GetNewPartitionInfo().GetSize(),
			Hash:                  part.GetNewPartitionInfo().GetHash(),
			OldSize:               part.GetOldPartitionInfo().GetSize(),
			OldHash:               part.GetOldPartitionInfo().GetHash(),
			NeedsSource:           partitionNeedsSource(part),
			TotalOperations:       len(part.GetOperations()),
			UnsupportedOperations: partitionUnsupportedOps(part),
		})
	}
	return parts
}

func operationNeedsSource(op *chromeos_update_engine.InstallOperation) bool {
	if len(op.GetSrcExtents()) > 0 {
		return true
	}
	switch op.GetType() {
	case chromeos_update_engine.InstallOperation_MOVE,
		chromeos_update_engine.InstallOperation_BSDIFF,
		chromeos_update_engine.InstallOperation_SOURCE_COPY,
		chromeos_update_engine.InstallOperation_SOURCE_BSDIFF,
		chromeos_update_engine.InstallOperation_BROTLI_BSDIFF,
		chromeos_update_engine.InstallOperation_PUFFDIFF,
		chromeos_update_engine.InstallOperation_ZUCCHINI,
		chromeos_update_engine.InstallOperation_LZ4DIFF_BSDIFF,
		chromeos_update_engine.InstallOperation_LZ4DIFF_PUFFDIFF:
		return true
	}
	return false
}

func operationSupported(t chromeos_update_engine.InstallOperation_Type) bool {
	switch t {
	case chromeos_update_engine.InstallOperation_REPLACE,
		chromeos_update_engine.InstallOperation_REPLACE_BZ,
		chromeos_update_engine.InstallOperation_REPLACE_XZ,
		chromeos_update_engine.InstallOperation_ZSTD,
		chromeos_update_engine.InstallOperation_ZERO,
		chromeos_update_engine.InstallOperation_DISCARD,
		chromeos_update_engine.InstallOperation_MOVE,
		chromeos_update_engine.InstallOperation_BSDIFF,
		chromeos_update_engine.InstallOperation_SOURCE_COPY,
		chromeos_update_engine.InstallOperation_SOURCE_BSDIFF,
		chromeos_update_engine.InstallOperation_BROTLI_BSDIFF:
		return true
	}
	return false
}

func partitionNeedsSource(part *chromeos_update_engine.PartitionUpdate) bool {
	for _, op := range part.GetOperations() {
		if operationNeedsSource(op) {
			return true
		}
	}
	return false
}

func partitionUnsupportedOps(part *chromeos_update_engine.PartitionUpdate) []string {
	seen := map[string]bool{}
	var out []string
	for _, op := range part.GetOperations() {
		if !operationSupported(op.GetType()) {
			name := op.GetType().String()
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
