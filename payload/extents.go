package payload

import (
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

const sparseHoleBlock = math.MaxUint64

func totalBlocks(extents []*chromeos_update_engine.Extent) uint64 {
	var n uint64
	for _, e := range extents {
		n += e.GetNumBlocks()
	}
	return n
}

func validateExtents(extents []*chromeos_update_engine.Extent, blockSize uint64) error {
	for _, e := range extents {
		start := e.GetStartBlock()
		num := e.GetNumBlocks()
		if start == sparseHoleBlock {
			return fmt.Errorf("payload: sparse hole extents are not supported")
		}
		if num == 0 {
			continue
		}
		end := start + num
		if end < start || end > math.MaxInt64/blockSize {
			return fmt.Errorf("payload: extent out of addressable range (start_block=%d num_blocks=%d)", start, num)
		}
	}
	return nil
}

type extentsReader struct {
	ra        io.ReaderAt
	extents   []*chromeos_update_engine.Extent
	blockSize uint64
	idx       int
	consumed  int64
}

func newExtentsReader(ra io.ReaderAt, extents []*chromeos_update_engine.Extent, blockSize uint64) (*extentsReader, error) {
	if err := validateExtents(extents, blockSize); err != nil {
		return nil, err
	}
	return &extentsReader{ra: ra, extents: extents, blockSize: blockSize}, nil
}

func (r *extentsReader) Read(p []byte) (int, error) {
	for r.idx < len(r.extents) {
		e := r.extents[r.idx]
		length := int64(e.GetNumBlocks() * r.blockSize)
		if r.consumed >= length {
			r.idx++
			r.consumed = 0
			continue
		}
		remain := length - r.consumed
		if int64(len(p)) > remain {
			p = p[:remain]
		}
		off := int64(e.GetStartBlock()*r.blockSize) + r.consumed
		n, err := r.ra.ReadAt(p, off)
		r.consumed += int64(n)
		if err == io.EOF && n > 0 {
			err = nil
		}
		if err == io.EOF {
			err = fmt.Errorf("payload: source data ends before extent (offset=%d length=%d): %w", off, remain, io.ErrUnexpectedEOF)
		}
		return n, err
	}
	return 0, io.EOF
}

type blockRange struct {
	start uint64
	end   uint64
}

func uncoveredRanges(extents []*chromeos_update_engine.Extent, totalBlocks uint64) []blockRange {
	covered := make([]blockRange, 0, len(extents))
	for _, e := range extents {
		if e.GetNumBlocks() == 0 {
			continue
		}
		covered = append(covered, blockRange{start: e.GetStartBlock(), end: e.GetStartBlock() + e.GetNumBlocks()})
	}
	sort.Slice(covered, func(i, j int) bool { return covered[i].start < covered[j].start })
	var gaps []blockRange
	cursor := uint64(0)
	for _, r := range covered {
		if r.start > cursor {
			end := r.start
			if end > totalBlocks {
				end = totalBlocks
			}
			if end > cursor {
				gaps = append(gaps, blockRange{start: cursor, end: end})
			}
		}
		if r.end > cursor {
			cursor = r.end
		}
		if cursor >= totalBlocks {
			return gaps
		}
	}
	if cursor < totalBlocks {
		gaps = append(gaps, blockRange{start: cursor, end: totalBlocks})
	}
	return gaps
}

type extentsWriter struct {
	w         io.WriterAt
	extents   []*chromeos_update_engine.Extent
	blockSize uint64
	idx       int
	written   int64
}

func newExtentsWriter(w io.WriterAt, extents []*chromeos_update_engine.Extent, blockSize uint64) (*extentsWriter, error) {
	if err := validateExtents(extents, blockSize); err != nil {
		return nil, err
	}
	return &extentsWriter{w: w, extents: extents, blockSize: blockSize}, nil
}

func (w *extentsWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		if w.idx >= len(w.extents) {
			return total, fmt.Errorf("payload: attempted to write past destination extents")
		}
		e := w.extents[w.idx]
		length := int64(e.GetNumBlocks() * w.blockSize)
		if w.written >= length {
			w.idx++
			w.written = 0
			continue
		}
		chunk := p
		remain := length - w.written
		if int64(len(chunk)) > remain {
			chunk = chunk[:remain]
		}
		off := int64(e.GetStartBlock()*w.blockSize) + w.written
		n, err := w.w.WriteAt(chunk, off)
		total += n
		w.written += int64(n)
		if err != nil {
			return total, err
		}
		p = p[n:]
	}
	return total, nil
}
