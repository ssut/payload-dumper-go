package payload

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func writeHashTree(out *os.File, part *chromeos_update_engine.PartitionUpdate, blockSize uint64, logger *slog.Logger) error {
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
	salt := part.GetHashTreeSalt()
	hashSize := nextPowerOfTwo(digestLen)
	dataOffset := int64(dataExtent.GetStartBlock() * blockSize)
	dataBlocks := dataExtent.GetNumBlocks()

	level := make([]byte, 0, (dataBlocks*uint64(hashSize)+blockSize-1)/blockSize*blockSize)
	buf := make([]byte, blockSize)
	digest := make([]byte, 0, digestLen)
	reader := io.NewSectionReader(out, dataOffset, int64(dataBlocks*blockSize))
	for i := uint64(0); i < dataBlocks; i++ {
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

func checkFecReconstructible(part *chromeos_update_engine.PartitionUpdate, blockSize uint64) error {
	fec := part.GetFecExtent()
	if fec == nil || fec.GetNumBlocks() == 0 {
		return nil
	}
	totalBlocks := part.GetNewPartitionInfo().GetSize() / blockSize
	var dst []*chromeos_update_engine.Extent
	for _, op := range part.GetOperations() {
		dst = append(dst, op.GetDstExtents()...)
	}
	fecStart := fec.GetStartBlock()
	fecEnd := fecStart + fec.GetNumBlocks()
	for _, gap := range uncoveredRanges(dst, totalBlocks) {
		if gap.start < fecEnd && fecStart < gap.end {
			return fmt.Errorf("payload: partition %q requires FEC (forward error correction) data reconstruction, which is not supported yet", part.GetPartitionName())
		}
	}
	return nil
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
