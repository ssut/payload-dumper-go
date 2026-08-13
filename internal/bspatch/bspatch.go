package bspatch

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/andybalholm/brotli"
)

const headerLen = 32

const (
	compressorBZ2    = 1
	compressorBrotli = 2
)

var (
	magicBSDIFF40 = []byte("BSDIFF40")
	magicBSDF2    = []byte("BSDF2")
)

func Apply(old, patch []byte, maxNewSize int64) ([]byte, error) {
	if len(patch) < headerLen {
		return nil, fmt.Errorf("bspatch: patch too small (%d bytes)", len(patch))
	}
	var ctrlType, diffType, extraType byte
	switch {
	case bytes.HasPrefix(patch, magicBSDIFF40):
		ctrlType, diffType, extraType = compressorBZ2, compressorBZ2, compressorBZ2
	case bytes.HasPrefix(patch, magicBSDF2):
		ctrlType, diffType, extraType = patch[5], patch[6], patch[7]
	default:
		return nil, fmt.Errorf("bspatch: unrecognized patch magic % x", patch[:8])
	}
	ctrlLen := parseInt64(patch[8:16])
	diffLen := parseInt64(patch[16:24])
	newSize := parseInt64(patch[24:32])
	if ctrlLen < 0 || diffLen < 0 || newSize < 0 ||
		int64(len(patch))-headerLen < ctrlLen ||
		int64(len(patch))-headerLen-ctrlLen < diffLen {
		return nil, fmt.Errorf("bspatch: corrupt patch header (ctrl=%d diff=%d new_size=%d patch=%d)", ctrlLen, diffLen, newSize, len(patch))
	}
	if newSize > maxNewSize {
		return nil, fmt.Errorf("bspatch: patch output size %d exceeds destination capacity %d", newSize, maxNewSize)
	}
	ctrl, err := newDecompressor(ctrlType, patch[headerLen:headerLen+ctrlLen])
	if err != nil {
		return nil, err
	}
	diff, err := newDecompressor(diffType, patch[headerLen+ctrlLen:headerLen+ctrlLen+diffLen])
	if err != nil {
		return nil, err
	}
	extra, err := newDecompressor(extraType, patch[headerLen+ctrlLen+diffLen:])
	if err != nil {
		return nil, err
	}

	newData := make([]byte, newSize)
	oldSize := int64(len(old))
	var oldpos, newpos int64
	ctrlBuf := make([]byte, 24)
	for newpos < newSize {
		if _, err := io.ReadFull(ctrl, ctrlBuf); err != nil {
			return nil, fmt.Errorf("bspatch: reading control stream: %w", err)
		}
		diffSize := parseInt64(ctrlBuf[0:8])
		extraSize := parseInt64(ctrlBuf[8:16])
		offsetIncrement := parseInt64(ctrlBuf[16:24])
		if diffSize < 0 || extraSize < 0 {
			return nil, errors.New("bspatch: corrupt patch (negative control entry sizes)")
		}
		if diffSize > newSize-newpos {
			return nil, errors.New("bspatch: corrupt patch (diff block writes past output end)")
		}
		if _, err := io.ReadFull(diff, newData[newpos:newpos+diffSize]); err != nil {
			return nil, fmt.Errorf("bspatch: reading diff stream: %w", err)
		}
		for i := int64(0); i < diffSize; i++ {
			if p := oldpos + i; p >= 0 && p < oldSize {
				newData[newpos+i] += old[p]
			}
		}
		newpos += diffSize
		if oldpos > math.MaxInt64-diffSize {
			return nil, errors.New("bspatch: corrupt patch (old position overflow)")
		}
		oldpos += diffSize
		if extraSize > newSize-newpos {
			return nil, errors.New("bspatch: corrupt patch (extra block writes past output end)")
		}
		if _, err := io.ReadFull(extra, newData[newpos:newpos+extraSize]); err != nil {
			return nil, fmt.Errorf("bspatch: reading extra stream: %w", err)
		}
		newpos += extraSize
		if offsetIncrement > 0 && oldpos > math.MaxInt64-offsetIncrement {
			return nil, errors.New("bspatch: corrupt patch (old position overflow)")
		}
		oldpos += offsetIncrement
	}
	return newData, nil
}

func newDecompressor(compressorType byte, data []byte) (io.Reader, error) {
	switch compressorType {
	case compressorBZ2:
		return bzip2.NewReader(bytes.NewReader(data)), nil
	case compressorBrotli:
		return brotli.NewReader(bytes.NewReader(data)), nil
	default:
		return nil, fmt.Errorf("bspatch: unsupported compressor type %d", compressorType)
	}
}

func parseInt64(b []byte) int64 {
	v := int64(binary.LittleEndian.Uint64(b) & math.MaxInt64)
	if b[7]&0x80 != 0 {
		v = -v
	}
	return v
}
