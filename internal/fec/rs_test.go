package fec

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/ssut/payload-dumper-go/internal/fec/fectest"
)

func TestGeneratorPoly(t *testing.T) {
	g := generatorPoly(2)
	if want := []byte{0x02, 0x03, 0x01}; !bytes.Equal(g, want) {
		t.Fatalf("generatorPoly(2) = %#v, want %#v", g, want)
	}
	g = generatorPoly(3)
	if want := []byte{0x08, 0x0e, 0x07, 0x01}; !bytes.Equal(g, want) {
		t.Fatalf("generatorPoly(3) = %#v, want %#v", g, want)
	}
	for roots := 1; roots <= 254; roots++ {
		g := generatorPoly(roots)
		if len(g) != roots+1 {
			t.Fatalf("roots=%d: generator has %d coefficients, want %d", roots, len(g), roots+1)
		}
		if g[roots] != 1 {
			t.Fatalf("roots=%d: generator is not monic, leading coefficient %#02x", roots, g[roots])
		}
	}
}

func TestEncodeAOSPVector(t *testing.T) {
	enc, err := NewEncoder(2)
	if err != nil {
		t.Fatal(err)
	}
	msg := make([]byte, enc.DataLen())
	msg[0] = 0x01

	parity := make([]byte, 2)
	enc.Encode(msg, parity)
	if want := []byte{0x8e, 0x8f}; !bytes.Equal(parity, want) {
		t.Fatalf("Encode = %#v, want %#v (AOSP verity_writer_android_unittest.cc FECTest)", parity, want)
	}

	states := make([]byte, 2)
	for _, s := range msg {
		enc.Update(states, []byte{s})
	}
	if !bytes.Equal(states, parity) {
		t.Fatalf("Update loop = %#v, Encode = %#v", states, parity)
	}
}

func TestTrailingZerosMatter(t *testing.T) {
	enc, err := NewEncoder(2)
	if err != nil {
		t.Fatal(err)
	}
	short := make([]byte, 2)
	enc.Encode([]byte{0x01}, short)
	if want := []byte{0x03, 0x02}; !bytes.Equal(short, want) {
		t.Fatalf("Encode([1]) = %#v, want %#v", short, want)
	}
	full := make([]byte, 2)
	msg := make([]byte, enc.DataLen())
	msg[0] = 0x01
	enc.Encode(msg, full)
	if bytes.Equal(short, full) {
		t.Fatal("trailing zero symbols did not change the parity; zero padding is being skipped")
	}
}

func TestNewEncoderRejectsRoots(t *testing.T) {
	for _, roots := range []int{-1, 0, 255, 256} {
		if _, err := NewEncoder(roots); err == nil {
			t.Errorf("NewEncoder(%d) succeeded, want error", roots)
		}
	}
}

func TestGFMulMatchesOracle(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			got := gfMul(byte(a), byte(b))
			if want := fectest.Mul(byte(a), byte(b)); got != want {
				t.Fatalf("gfMul(%#02x, %#02x) = %#02x, want %#02x", a, b, got, want)
			}
		}
	}
	for _, tc := range []struct {
		n    int
		want byte
	}{{0, 0x01}, {1, 0x02}, {8, 0x1d}, {254, 0x8e}, {255, 0x01}} {
		if got := fectest.Exp(tc.n); got != tc.want {
			t.Errorf("alpha^%d = %#02x, want %#02x", tc.n, got, tc.want)
		}
	}
}

func TestGeneratorPolyMatchesOracle(t *testing.T) {
	for roots := 1; roots <= 254; roots++ {
		got := generatorPoly(roots)
		want := fectest.Gen(roots)
		for i := range want {
			if got[roots-i] != want[i] {
				t.Fatalf("roots=%d: production g[%d]=%#02x, oracle g[%d]=%#02x", roots, roots-i, got[roots-i], i, want[i])
			}
		}
	}
}

func testMessages(roots int, rnd *rand.Rand) [][]byte {
	n := 255 - roots
	msgs := [][]byte{
		make([]byte, n),
		make([]byte, n),
		make([]byte, n),
		make([]byte, n),
	}
	msgs[1][0] = 0x01
	msgs[2][n-1] = 0x01
	for i := range msgs[3] {
		msgs[3][i] = byte(i*131 + i>>3)
	}
	for i := 0; i < 16; i++ {
		m := make([]byte, n)
		rnd.Read(m)
		msgs = append(msgs, m)
	}
	return msgs
}

func TestEncodeSatisfiesRootProperty(t *testing.T) {
	rnd := rand.New(rand.NewSource(0x0FEC))
	for roots := 2; roots <= 24; roots++ {
		enc, err := NewEncoder(roots)
		if err != nil {
			t.Fatal(err)
		}
		for mi, msg := range testMessages(roots, rnd) {
			parity := make([]byte, roots)
			enc.Encode(msg, parity)
			if mi == 0 {
				for _, b := range parity {
					if b != 0 {
						t.Fatalf("roots=%d: all-zero message produced non-zero parity %#v", roots, parity)
					}
				}
			}
			cw := append(append([]byte{}, msg...), parity...)
			for i := 0; i < roots; i++ {
				if got := fectest.Eval(cw, fectest.Exp(i)); got != 0 {
					t.Fatalf("roots=%d msg#%d: c(alpha^%d) = %#02x, want 0", roots, mi, i, got)
				}
			}
			if mi == 0 {
				continue
			}
			cw[len(cw)-1] ^= 0x01
			broken := false
			for i := 0; i < roots; i++ {
				if fectest.Eval(cw, fectest.Exp(i)) != 0 {
					broken = true
					break
				}
			}
			if !broken {
				t.Fatalf("roots=%d msg#%d: root check is vacuous, corrupted parity still passed", roots, mi)
			}
		}
	}
}

func TestEncodeMatchesOracle(t *testing.T) {
	rnd := rand.New(rand.NewSource(0x0FEC))
	for roots := 2; roots <= 24; roots++ {
		enc, err := NewEncoder(roots)
		if err != nil {
			t.Fatal(err)
		}
		parity := make([]byte, roots)
		for mi, msg := range testMessages(roots, rnd) {
			enc.Encode(msg, parity)
			if want := fectest.Encode(msg, roots); !bytes.Equal(parity, want) {
				t.Fatalf("roots=%d msg#%d: production %#v, oracle %#v", roots, mi, parity, want)
			}
		}
	}
}
