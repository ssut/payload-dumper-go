package fec

import (
	"bytes"
	"context"
	"io"
	"testing"

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

func runGenerate(t *testing.T, data []byte, roots, workers int) []byte {
	t.Helper()
	p := Params{DataBlocks: len(data) / BlockSize, Roots: roots, BlockSize: BlockSize}
	f := &memFile{b: make([]byte, int64(len(data))+p.ParityBytes())}
	copy(f.b, data)
	if err := Generate(context.Background(), f, f, 0, int64(len(data)), p, workers, nil, nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return f.b[len(data):]
}

func TestFecAOSPPattern(t *testing.T) {
	data := bytes.Repeat([]byte{0x01}, BlockSize)
	got := runGenerate(t, data, 2, 1)
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
	shapes := []struct{ dataBlocks, roots int }{
		{1, 2}, {253, 2}, {254, 2}, {506, 2}, {507, 2},
		{100, 24}, {232, 24}, {60, 10}, {130, 12},
	}
	for _, s := range shapes {
		data := patterned(s.dataBlocks * BlockSize)
		got := runGenerate(t, data, s.roots, 4)
		want := fectest.FEC(data, s.roots)
		if len(got) != len(want) {
			t.Fatalf("dataBlocks=%d roots=%d: parity length %d, want %d", s.dataBlocks, s.roots, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				p := Params{DataBlocks: s.dataBlocks, Roots: s.roots, BlockSize: BlockSize}
				t.Fatalf("dataBlocks=%d roots=%d rounds=%d: first difference at parity byte %d: got %#02x, want %#02x",
					s.dataBlocks, s.roots, p.Rounds(), i, got[i], want[i])
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
	got := runGenerate(t, data, 2, 1)
	padded := append(append([]byte{}, data...), make([]byte, 252*BlockSize)...)
	gotPadded := runGenerate(t, padded, 2, 1)
	if !bytes.Equal(got, gotPadded) {
		t.Fatal("explicit trailing zero blocks changed the parity; out-of-range blocks are not treated as zero symbols")
	}
}

func TestFecDeterministicAcrossWorkers(t *testing.T) {
	data := patterned(600 * BlockSize)
	base := runGenerate(t, data, 2, 1)
	for _, workers := range []int{2, 4, 8, 16} {
		if got := runGenerate(t, data, 2, workers); !bytes.Equal(got, base) {
			t.Fatalf("workers=%d produced different parity than workers=1", workers)
		}
	}
}

func TestFecContextCancel(t *testing.T) {
	data := patterned(600 * BlockSize)
	p := Params{DataBlocks: 600, Roots: 2, BlockSize: BlockSize}
	f := &memFile{b: make([]byte, int64(len(data))+p.ParityBytes())}
	copy(f.b, data)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Generate(ctx, f, f, 0, int64(len(data)), p, 4, nil, nil)
	if err == nil {
		t.Fatal("Generate succeeded with a cancelled context")
	}
	if ctx.Err() == nil {
		t.Fatal("context was not cancelled")
	}
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
