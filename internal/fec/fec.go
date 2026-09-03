package fec

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

const (
	BlockSize          = 4096
	DefaultBatchRounds = 128
)

type Limiter interface {
	Acquire(ctx context.Context, n int64) error
	Release(n int64)
}

type Params struct {
	DataBlocks  int
	Roots       int
	BlockSize   uint64
	BatchRounds int
}

func (p Params) rsN() int { return 255 - p.Roots }

func (p Params) Rounds() int {
	n := p.rsN()
	return (p.DataBlocks + n - 1) / n
}

func (p Params) batchRounds() int {
	if p.BatchRounds <= 0 {
		return DefaultBatchRounds
	}
	return p.BatchRounds
}

func (p Params) Batches() int {
	b := p.batchRounds()
	return (p.Rounds() + b - 1) / b
}

func (p Params) ParityBytes() int64 {
	return int64(p.Rounds()) * int64(p.Roots) * int64(p.BlockSize)
}

func (p Params) Validate() error {
	if p.BlockSize != BlockSize {
		return fmt.Errorf("fec: block size must be %d, got %d", BlockSize, p.BlockSize)
	}
	if p.Roots < 1 || p.Roots > 254 {
		return fmt.Errorf("fec: roots must be in 1..254, got %d", p.Roots)
	}
	if p.DataBlocks <= 0 {
		return fmt.Errorf("fec: data block count must be positive, got %d", p.DataBlocks)
	}
	if p.BatchRounds < 0 {
		return fmt.Errorf("fec: batch rounds must not be negative, got %d", p.BatchRounds)
	}
	return nil
}

func Generate(ctx context.Context, src io.ReaderAt, dst io.WriterAt, dataOffset, fecOffset int64, p Params, workers int, lim Limiter, progress func(done, total int)) error {
	if err := p.Validate(); err != nil {
		return err
	}
	enc, err := NewEncoder(p.Roots)
	if err != nil {
		return err
	}
	rounds := p.Rounds()
	batches := p.Batches()
	batchRounds := p.batchRounds()
	if workers <= 0 {
		workers = 1
	}
	if workers > batches {
		workers = batches
	}

	var next, done atomic.Int64
	g, ctx := errgroup.WithContext(ctx)
	for w := 0; w < workers; w++ {
		g.Go(func() error {
			var buf, state []byte
			for {
				batch := int(next.Add(1) - 1)
				if batch >= batches {
					return nil
				}
				if lim != nil {
					if err := lim.Acquire(ctx, 1); err != nil {
						return err
					}
				}
				if buf == nil {
					buf = make([]byte, batchRounds*int(p.BlockSize))
					state = make([]byte, batchRounds*int(p.BlockSize)*p.Roots)
				}
				err := generateBatch(ctx, src, dst, enc, batch, rounds, p, dataOffset, fecOffset, buf, state)
				if lim != nil {
					lim.Release(1)
				}
				if err != nil {
					return err
				}
				if progress != nil {
					progress(int(done.Add(1)), batches)
				}
			}
		})
	}
	return g.Wait()
}

func generateBatch(ctx context.Context, src io.ReaderAt, dst io.WriterAt, enc *Encoder, batch, rounds int, p Params, dataOffset, fecOffset int64, buf, state []byte) error {
	bs := int(p.BlockSize)
	width := p.batchRounds()
	r0 := batch * width
	if rounds-r0 < width {
		width = rounds - r0
	}
	span := width * bs
	syms := buf[:span]
	st := state[:span*p.Roots]
	clear(st)

	for j := 0; j < enc.DataLen(); j++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		first := j*rounds + r0
		have := 0
		if first < p.DataBlocks {
			have = width
			if p.DataBlocks-first < have {
				have = p.DataBlocks - first
			}
			if _, err := src.ReadAt(syms[:have*bs], dataOffset+int64(first)*int64(bs)); err != nil {
				return fmt.Errorf("fec: reading data block %d: %w", first, err)
			}
		}
		if have < width {
			clear(syms[have*bs:])
		}
		enc.Update(st, syms)
	}

	if _, err := dst.WriteAt(st, fecOffset+int64(r0)*int64(p.Roots)*int64(bs)); err != nil {
		return fmt.Errorf("fec: writing parity for round %d: %w", r0, err)
	}
	return nil
}
