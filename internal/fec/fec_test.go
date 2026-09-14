package fec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"golang.org/x/sync/semaphore"

	"github.com/ssut/payload-dumper-go/internal/fec/fectest"
)

type memFile struct{ b []byte }

func (m *memFile) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(m.b)) {
		return 0, io.EOF
	}
	n := copy(p, m.b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (m *memFile) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || off+int64(len(p)) > int64(len(m.b)) {
		return 0, io.ErrShortWrite
	}
	copy(m.b[off:], p)
	return len(p), nil
}

func patterned(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*131 + i>>13)
	}
	return b
}

func params(data []byte, roots, batchRounds int) Params {
	return Params{DataBlocks: len(data) / BlockSize, Roots: roots, BlockSize: BlockSize, BatchRounds: batchRounds}
}

type generateRun struct {
	parity     []byte
	batches    int
	done       []int
	wrongTotal int
}

func generateWith(t *testing.T, ctx context.Context, data []byte, p Params, workers int, hook func(done, total int)) (generateRun, error) {
	t.Helper()
	f := &memFile{b: make([]byte, int64(len(data))+p.ParityBytes())}
	copy(f.b, data)
	run := generateRun{batches: p.Batches()}
	var mu sync.Mutex
	progress := func(done, total int) {
		mu.Lock()
		run.done = append(run.done, done)
		if total != run.batches {
			run.wrongTotal++
		}
		mu.Unlock()
		if hook != nil {
			hook(done, total)
		}
	}
	err := Generate(ctx, f, f, 0, int64(len(data)), p, workers, nil, progress)
	run.parity = f.b[len(data):]
	return run, err
}

func checkProgress(t *testing.T, run generateRun) {
	t.Helper()
	if run.wrongTotal > 0 {
		t.Fatalf("%d progress calls reported a total other than %d batches", run.wrongTotal, run.batches)
	}
	if len(run.done) != run.batches {
		t.Fatalf("progress reported %d times for %d batches: %v", len(run.done), run.batches, run.done)
	}
	seen := make([]bool, run.batches+1)
	for _, d := range run.done {
		if d < 1 || d > run.batches {
			t.Fatalf("progress value %d out of range 1..%d", d, run.batches)
		}
		if seen[d] {
			t.Fatalf("progress value %d reported twice: %v", d, run.done)
		}
		seen[d] = true
	}
}

func runGenerate(t *testing.T, data []byte, roots, workers, batchRounds int) []byte {
	t.Helper()
	run, err := generateWith(t, context.Background(), data, params(data, roots, batchRounds), workers, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checkProgress(t, run)
	return run.parity
}

func TestFecAOSPPattern(t *testing.T) {
	data := bytes.Repeat([]byte{0x01}, BlockSize)
	got := runGenerate(t, data, 2, 1, 0)
	want := bytes.Repeat([]byte{0x8e, 0x8f}, BlockSize)
	if !bytes.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("parity differs at byte %d: got %#02x, want %#02x (len %d, want len %d)", i, got[i], want[i], len(got), len(want))
			}
		}
		t.Fatalf("parity length %d, want %d", len(got), len(want))
	}
}

func TestFecMatchesOracle(t *testing.T) {
	shapes := []struct{ dataBlocks, roots, workers, batch int }{
		{1, 2, 1, 0}, {253, 2, 4, 0}, {254, 2, 4, 0}, {506, 2, 4, 0}, {507, 2, 4, 0},
		{100, 24, 4, 0}, {232, 24, 4, 0}, {60, 10, 4, 0}, {130, 12, 4, 0},
		{254, 2, 2, 1}, {507, 2, 3, 1}, {507, 2, 2, 2}, {1000, 2, 4, 3}, {232, 24, 2, 1}, {730, 12, 4, 2},
	}
	for _, s := range shapes {
		data := patterned(s.dataBlocks * BlockSize)
		p := params(data, s.roots, s.batch)
		if s.batch > 0 && (p.Batches() < 2 || s.workers < 2) {
			t.Fatalf("shape %+v spans %d batches with %d workers; it does not exercise the parallel multi-batch path", s, p.Batches(), s.workers)
		}
		got := runGenerate(t, data, s.roots, s.workers, s.batch)
		want := fectest.FEC(data, s.roots)
		if len(got) != len(want) {
			t.Fatalf("shape %+v: parity length %d, want %d", s, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("shape %+v rounds=%d batches=%d: first difference at parity byte %d: got %#02x, want %#02x",
					s, p.Rounds(), p.Batches(), i, got[i], want[i])
			}
		}
	}
}

func TestInterleaveMatchesUpstreamFormula(t *testing.T) {
	for _, roots := range []int{2, 8, 24} {
		rsn := 255 - roots
		for _, rounds := range []int{1, 2, 3, 7, 100} {
			for _, i := range []int{0, 1, rounds - 1} {
				if i < 0 {
					continue
				}
				for _, j := range []int{0, 1, rsn / 2, rsn - 1} {
					off := int64(i)*int64(rsn)*BlockSize + int64(j)
					want := fectest.Interleave(off, rsn, int64(rounds))
					got := int64(i+j*rounds) * BlockSize
					if got != want {
						t.Fatalf("roots=%d rounds=%d i=%d j=%d: simplified %d, upstream %d", roots, rounds, i, j, got, want)
					}
				}
			}
		}
	}
}

func TestFecZeroPadIdentity(t *testing.T) {
	data := patterned(254 * BlockSize)
	got := runGenerate(t, data, 2, 1, 0)
	padded := append(append([]byte{}, data...), make([]byte, 252*BlockSize)...)
	gotPadded := runGenerate(t, padded, 2, 1, 0)
	if !bytes.Equal(got, gotPadded) {
		t.Fatal("explicit trailing zero blocks changed the parity; out-of-range blocks are not treated as zero symbols")
	}
}

func TestFecDeterministicAcrossWorkers(t *testing.T) {
	data := patterned(8 * 253 * BlockSize)
	if p := params(data, 2, 1); p.Batches() != 8 {
		t.Fatalf("fixture spans %d batches, want 8", p.Batches())
	}
	base := runGenerate(t, data, 2, 1, 1)
	if want := fectest.FEC(data, 2); !bytes.Equal(base, want) {
		t.Fatal("single-worker parity does not match the reference implementation")
	}
	for _, workers := range []int{2, 4, 8, 16} {
		if got := runGenerate(t, data, 2, workers, 1); !bytes.Equal(got, base) {
			t.Fatalf("workers=%d produced different parity than workers=1", workers)
		}
	}
	if got := runGenerate(t, data, 2, 4, 0); !bytes.Equal(got, base) {
		t.Fatal("default batch size produced different parity than single-round batches")
	}
}

func TestFecContextCancel(t *testing.T) {
	t.Run("before-start", func(t *testing.T) {
		data := patterned(600 * BlockSize)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		run, err := generateWith(t, ctx, data, params(data, 2, 0), 4, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Generate returned %v, want context.Canceled", err)
		}
		if len(run.done) != 0 {
			t.Fatalf("%d batches completed with a cancelled context", len(run.done))
		}
	})
	t.Run("mid-run", func(t *testing.T) {
		data := patterned(8 * 253 * BlockSize)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		run, err := generateWith(t, ctx, data, params(data, 2, 1), 2, func(done, total int) { cancel() })
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Generate returned %v, want context.Canceled", err)
		}
		if len(run.done) == 0 || len(run.done) >= run.batches {
			t.Fatalf("%d of %d batches completed, want at least one but not all", len(run.done), run.batches)
		}
	})
}

func TestFecValidation(t *testing.T) {
	cases := []struct {
		name string
		p    Params
	}{
		{"block-size", Params{DataBlocks: 1, Roots: 2, BlockSize: 8192}},
		{"roots-zero", Params{DataBlocks: 1, Roots: 0, BlockSize: BlockSize}},
		{"roots-255", Params{DataBlocks: 1, Roots: 255, BlockSize: BlockSize}},
		{"no-data", Params{DataBlocks: 0, Roots: 2, BlockSize: BlockSize}},
		{"batch-negative", Params{DataBlocks: 1, Roots: 2, BlockSize: BlockSize, BatchRounds: -1}},
		{"address-overflow", Params{DataBlocks: int(^uint(0) >> 1), Roots: 2, BlockSize: BlockSize, BatchRounds: int(^uint(0) >> 1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &memFile{b: make([]byte, BlockSize*8)}
			if err := Generate(context.Background(), f, f, 0, 0, tc.p, 1, nil, nil); err == nil {
				t.Fatal("Generate succeeded, want error")
			}
		})
	}
}

type countingLimiter struct {
	sem      *semaphore.Weighted
	acquired int
	released int
}

func (l *countingLimiter) Acquire(ctx context.Context, n int64) error {
	if err := l.sem.Acquire(ctx, n); err != nil {
		return err
	}
	l.acquired++
	return nil
}

func (l *countingLimiter) Release(n int64) {
	l.released++
	l.sem.Release(n)
}

func TestFecLimiterCoversWorkerLifetime(t *testing.T) {
	data := patterned(4 * 253 * BlockSize)
	p := params(data, 2, 1)
	for _, failRead := range []bool{false, true} {
		t.Run(fmt.Sprint(failRead), func(t *testing.T) {
			src := &memFile{b: data}
			if failRead {
				src.b = nil
			}
			dst := &memFile{b: make([]byte, p.ParityBytes())}
			lim := &countingLimiter{sem: semaphore.NewWeighted(1)}
			err := Generate(context.Background(), src, dst, 0, 0, p, 1, lim, nil)
			if failRead && err == nil {
				t.Fatal("Generate succeeded with missing data")
			}
			if !failRead && err != nil {
				t.Fatal(err)
			}
			if lim.acquired != 1 || lim.released != 1 {
				t.Fatalf("limiter acquired %d times and released %d times, want one worker lifetime", lim.acquired, lim.released)
			}
			if !lim.sem.TryAcquire(1) {
				t.Fatal("worker leaked limiter capacity")
			}
			if !failRead && !bytes.Equal(dst.b, fectest.FEC(data, 2)) {
				t.Fatal("limited generation differs from oracle")
			}
		})
	}
}

func TestFecCeilDivisionDoesNotOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	p := Params{DataBlocks: maxInt, Roots: 2, BlockSize: BlockSize, BatchRounds: maxInt}
	want := maxInt / 253
	if maxInt%253 != 0 {
		want++
	}
	if got := p.Rounds(); got != want {
		t.Fatalf("rounds=%d, want %d", got, want)
	}
	if got := p.Batches(); got != 1 {
		t.Fatalf("batches=%d, want 1", got)
	}
}

func TestFecOversizedBatchUsesActualRounds(t *testing.T) {
	data := patterned(BlockSize)
	p := params(data, 2, int(^uint(0)>>1))
	run, err := generateWith(t, context.Background(), data, p, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(run.parity, fectest.FEC(data, 2)) {
		t.Fatal("parity differs from oracle")
	}
}

type boundedZeroReader struct{ size int64 }

func (r boundedZeroReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off > r.size-int64(len(p)) {
		return 0, io.EOF
	}
	clear(p)
	return len(p), nil
}

func TestFecLastRoundNear32BitBlockLimit(t *testing.T) {
	p := Params{DataBlocks: int(^uint32(0) >> 1), Roots: 2, BlockSize: BlockSize, BatchRounds: 1}
	enc, err := NewEncoder(p.Roots)
	if err != nil {
		t.Fatal(err)
	}
	dst := &memFile{b: make([]byte, BlockSize*p.Roots)}
	last := p.Rounds() - 1
	err = generateBatch(context.Background(), boundedZeroReader{size: int64(p.DataBlocks) * BlockSize}, dst, enc, last, p.Rounds(), p, 0, -int64(last)*int64(p.Roots)*BlockSize, make([]byte, BlockSize), make([]byte, BlockSize*p.Roots))
	if err != nil {
		t.Fatalf("last padded round overflowed its block index: %v", err)
	}
}
