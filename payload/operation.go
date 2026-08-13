package payload

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"github.com/ssut/payload-dumper-go/internal/bspatch"
)

const maxInMemoryOpBytes = int64(2) << 30

type partitionExtractor struct {
	p          *Payload
	name       string
	out        *os.File
	src        *sourceImage
	skipVerify bool
}

func (x *partitionExtractor) runOp(opIndex int, op *chromeos_update_engine.InstallOperation) error {
	var err error
	switch op.GetType() {
	case chromeos_update_engine.InstallOperation_REPLACE,
		chromeos_update_engine.InstallOperation_REPLACE_BZ,
		chromeos_update_engine.InstallOperation_REPLACE_XZ,
		chromeos_update_engine.InstallOperation_ZSTD:
		err = x.runReplace(op)
	case chromeos_update_engine.InstallOperation_ZERO,
		chromeos_update_engine.InstallOperation_DISCARD:
		err = x.runZero(op)
	case chromeos_update_engine.InstallOperation_SOURCE_COPY,
		chromeos_update_engine.InstallOperation_MOVE:
		err = x.runSourceCopy(op)
	case chromeos_update_engine.InstallOperation_SOURCE_BSDIFF,
		chromeos_update_engine.InstallOperation_BSDIFF,
		chromeos_update_engine.InstallOperation_BROTLI_BSDIFF:
		err = x.runBsdiff(op)
	default:
		err = &UnsupportedOperationsError{Partitions: map[string][]string{x.name: {op.GetType().String()}}}
	}
	if err != nil {
		return fmt.Errorf("partition %q operation %d (%s): %w", x.name, opIndex, op.GetType(), err)
	}
	return nil
}

func (x *partitionExtractor) blobSectionReader(op *chromeos_update_engine.InstallOperation) *io.SectionReader {
	return io.NewSectionReader(x.p.ra, x.p.dataOffset+int64(op.GetDataOffset()), int64(op.GetDataLength()))
}

func (x *partitionExtractor) readBlob(op *chromeos_update_engine.InstallOperation) ([]byte, error) {
	length := int64(op.GetDataLength())
	if length > maxInMemoryOpBytes {
		return nil, fmt.Errorf("operation data too large to buffer (%d bytes)", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(x.blobSectionReader(op), buf); err != nil {
		return nil, fmt.Errorf("reading operation data: %w", err)
	}
	if err := x.verifyBlobHash(op, sha256.Sum256(buf)); err != nil {
		return nil, err
	}
	return buf, nil
}

func (x *partitionExtractor) verifyBlobHash(op *chromeos_update_engine.InstallOperation, sum [32]byte) error {
	expected := op.GetDataSha256Hash()
	if len(expected) == 0 {
		return nil
	}
	if !hashEqual(sum[:], expected) {
		return &VerificationError{
			Partition: x.name,
			Subject:   "operation data",
			Expected:  hex.EncodeToString(expected),
			Actual:    hex.EncodeToString(sum[:]),
		}
	}
	return nil
}

func (x *partitionExtractor) runReplace(op *chromeos_update_engine.InstallOperation) error {
	dst := op.GetDstExtents()
	if len(dst) == 0 {
		return fmt.Errorf("missing dst_extents")
	}
	expected := int64(totalBlocks(dst) * x.p.blockSize)
	w, err := newExtentsWriter(x.out, dst, x.p.blockSize)
	if err != nil {
		return err
	}
	h := sha256.New()
	tee := io.TeeReader(x.blobSectionReader(op), h)
	var rd io.ReadCloser
	switch op.GetType() {
	case chromeos_update_engine.InstallOperation_REPLACE:
		rd = readCloser{Reader: tee}
	case chromeos_update_engine.InstallOperation_REPLACE_BZ:
		rd = newBzip2Reader(tee)
	case chromeos_update_engine.InstallOperation_REPLACE_XZ:
		rd = newXZReader(tee)
	case chromeos_update_engine.InstallOperation_ZSTD:
		rd = newZstdReader(tee)
	}
	n, err := io.Copy(w, rd)
	rd.Close()
	if err != nil {
		return fmt.Errorf("decompressing data: %w", err)
	}
	if n != expected {
		return fmt.Errorf("wrote %d bytes, destination extents expect %d", n, expected)
	}
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return fmt.Errorf("draining operation data: %w", err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return x.verifyBlobHash(op, sum)
}

func (x *partitionExtractor) runZero(op *chromeos_update_engine.InstallOperation) error {
	dst := op.GetDstExtents()
	if len(dst) == 0 {
		return fmt.Errorf("missing dst_extents")
	}
	expected := int64(totalBlocks(dst) * x.p.blockSize)
	w, err := newExtentsWriter(x.out, dst, x.p.blockSize)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(w, zeroReader{}, expected); err != nil {
		return fmt.Errorf("writing zeros: %w", err)
	}
	return nil
}

func (x *partitionExtractor) runSourceCopy(op *chromeos_update_engine.InstallOperation) error {
	if x.src == nil {
		return &MissingSourceError{Partitions: []string{x.name}}
	}
	src := op.GetSrcExtents()
	dst := op.GetDstExtents()
	if len(src) == 0 || len(dst) == 0 {
		return fmt.Errorf("missing src_extents or dst_extents")
	}
	srcBytes := int64(totalBlocks(src) * x.p.blockSize)
	dstBytes := int64(totalBlocks(dst) * x.p.blockSize)
	if srcBytes != dstBytes {
		return fmt.Errorf("source span (%d bytes) does not match destination span (%d bytes)", srcBytes, dstBytes)
	}
	r, err := newExtentsReader(x.src.f, src, x.p.blockSize)
	if err != nil {
		return err
	}
	w, err := newExtentsWriter(x.out, dst, x.p.blockSize)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(w, io.TeeReader(r, h))
	if err != nil {
		return fmt.Errorf("copying source extents: %w", err)
	}
	if n != dstBytes {
		return fmt.Errorf("copied %d bytes, expected %d", n, dstBytes)
	}
	return x.verifySourceHash(op, h.Sum(nil))
}

func (x *partitionExtractor) runBsdiff(op *chromeos_update_engine.InstallOperation) error {
	if x.src == nil {
		return &MissingSourceError{Partitions: []string{x.name}}
	}
	src := op.GetSrcExtents()
	dst := op.GetDstExtents()
	if len(src) == 0 || len(dst) == 0 {
		return fmt.Errorf("missing src_extents or dst_extents")
	}
	srcBytes := int64(totalBlocks(src) * x.p.blockSize)
	if srcBytes > maxInMemoryOpBytes {
		return fmt.Errorf("source span too large to buffer (%d bytes)", srcBytes)
	}
	r, err := newExtentsReader(x.src.f, src, x.p.blockSize)
	if err != nil {
		return err
	}
	srcData := make([]byte, srcBytes)
	if _, err := io.ReadFull(r, srcData); err != nil {
		return fmt.Errorf("reading source extents: %w", err)
	}
	srcSum := sha256.Sum256(srcData)
	if err := x.verifySourceHash(op, srcSum[:]); err != nil {
		return err
	}
	patch, err := x.readBlob(op)
	if err != nil {
		return err
	}
	dstBytes := int64(totalBlocks(dst) * x.p.blockSize)
	newData, err := bspatch.Apply(srcData, patch, dstBytes)
	if err != nil {
		return fmt.Errorf("applying bsdiff patch: %w", err)
	}
	w, err := newExtentsWriter(x.out, dst, x.p.blockSize)
	if err != nil {
		return err
	}
	if _, err := w.Write(newData); err != nil {
		return fmt.Errorf("writing patched data: %w", err)
	}
	return nil
}

func (x *partitionExtractor) verifySourceHash(op *chromeos_update_engine.InstallOperation, sum []byte) error {
	expected := op.GetSrcSha256Hash()
	if x.skipVerify || len(expected) == 0 {
		return nil
	}
	if !hashEqual(sum, expected) {
		return &VerificationError{
			Partition: x.name,
			Subject:   "source data",
			Expected:  hex.EncodeToString(expected),
			Actual:    hex.EncodeToString(sum),
		}
	}
	return nil
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
