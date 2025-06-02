package payload

import (
	"bytes"
	"compress/bzip2"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/spencercw/go-xz"
	"github.com/valyala/gozstd"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

// Extractor handles the extraction of partitions from payload files
type Extractor struct {
	file       *os.File
	dataOffset int64
}

// NewExtractor creates a new Extractor instance
func NewExtractor(file *os.File, dataOffset int64) *Extractor {
	return &Extractor{
		file:       file,
		dataOffset: dataOffset,
	}
}

// Extract extracts a partition to the given writer
func (e *Extractor) Extract(partition *chromeos_update_engine.PartitionUpdate, out io.Writer, progressBar ProgressBar) error {
	name := partition.GetPartitionName()

	for _, operation := range partition.Operations {
		if len(operation.DstExtents) == 0 {
			return fmt.Errorf("invalid operation.DstExtents for the partition %s", name)
		}
		
		if progressBar != nil {
			progressBar.Increment()
		}

		extent := operation.DstExtents[0]
		dataOffset := e.dataOffset + int64(operation.GetDataOffset())
		dataLength := int64(operation.GetDataLength())
		
		// Seek to the correct position in output
		if seeker, ok := out.(io.Seeker); ok {
			_, err := seeker.Seek(int64(extent.GetStartBlock())*BlockSize, 0)
			if err != nil {
				return err
			}
		}
		
		expectedUncompressedBlockSize := int64(extent.GetNumBlocks() * BlockSize)

		bufSha := sha256.New()
		teeReader := io.TeeReader(io.NewSectionReader(e.file, dataOffset, dataLength), bufSha)

		var err error
		var n int64

		switch operation.GetType() {
		case chromeos_update_engine.InstallOperation_REPLACE:
			n, err = io.Copy(out, teeReader)
			if err != nil {
				return err
			}

		case chromeos_update_engine.InstallOperation_REPLACE_XZ:
			reader := xz.NewDecompressionReader(teeReader)
			n, err = io.Copy(out, &reader)
			if err != nil {
				return err
			}
			reader.Close()

		case chromeos_update_engine.InstallOperation_REPLACE_BZ:
			reader := bzip2.NewReader(teeReader)
			n, err = io.Copy(out, reader)
			if err != nil {
				return err
			}

		case chromeos_update_engine.InstallOperation_ZSTD:
			reader := gozstd.NewReader(teeReader)
			n, err = io.Copy(out, reader)
			if err != nil {
				return err
			}

		case chromeos_update_engine.InstallOperation_ZERO:
			reader := bytes.NewReader(make([]byte, expectedUncompressedBlockSize))
			n, err = io.Copy(out, reader)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unhandled operation type: %s", operation.GetType().String())
		}

		if n != expectedUncompressedBlockSize {
			return fmt.Errorf("verify failed (unexpected bytes written): %s (%d != %d)", name, n, expectedUncompressedBlockSize)
		}

		// Verify hash
		hash := hex.EncodeToString(bufSha.Sum(nil))
		expectedHash := hex.EncodeToString(operation.GetDataSha256Hash())
		if expectedHash != "" && hash != expectedHash {
			return fmt.Errorf("verify failed (checksum mismatch): %s (%s != %s)", name, hash, expectedHash)
		}
	}

	return nil
}