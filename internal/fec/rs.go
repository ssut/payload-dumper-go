// Package fec implements the dm-verity forward error correction (FEC)
// encoding that Android's update_engine applies after installing a delta
// payload.
//
// The Reed-Solomon code parameters are fixed by Android's verity FEC format
// (FEC_PARAMS in AOSP libfec's include/fec/ecc.h, an Apache-2.0 header):
//
//	symbol size   8 bits, field GF(2^8) modulo 0x11d (x^8+x^4+x^3+x^2+1)
//	generator     g(x) = (x - a^0)(x - a^1)...(x - a^(roots-1)), a = 0x02
//	code          RS(255, 255-roots), systematic, no padding
//
// A systematic codeword is c(x) = d(x)*x^roots + p(x), where the parity p is
// the remainder of d(x)*x^roots divided by g(x). That remainder is unique, so
// any correct encoder using these parameters produces byte-identical parity.
// Parity is emitted most-significant coefficient first: p[0] is the
// coefficient of x^(roots-1), matching the on-disk order dm-verity expects.
//
// This file implements that definition directly: polynomial long division in
// GF(2^8), realized as a shift register whose taps are the coefficients of g.
//
// Phil Karn's Reed-Solomon implementation in AOSP external/fec is LGPL
// licensed, while this project is Apache-2.0. No code from it was copied.
// Only the parameter values above were taken, and those come from the
// Apache-2.0 licensed libfec ecc.h rather than from the LGPL sources.
//
// References:
//
//	https://android.googlesource.com/platform/system/extras/+/refs/heads/main/libfec/include/fec/ecc.h
//	https://android.googlesource.com/platform/system/update_engine/+/refs/heads/main/payload_consumer/verity_writer_android.cc
//	https://docs.kernel.org/admin-guide/device-mapper/verity.html
package fec

import "fmt"

const (
	fieldPoly    = 0x11d
	fieldPolyLow = byte(fieldPoly & 0xff)
)

func gfMul(a, b byte) byte {
	var p byte
	for b != 0 {
		if b&1 != 0 {
			p ^= a
		}
		b >>= 1
		carry := a & 0x80
		a <<= 1
		if carry != 0 {
			a ^= fieldPolyLow
		}
	}
	return p
}

func generatorPoly(roots int) []byte {
	g := make([]byte, roots+1)
	g[0] = 1
	root := byte(1)
	for i := 0; i < roots; i++ {
		for k := i + 1; k > 0; k-- {
			g[k] = g[k-1] ^ gfMul(g[k], root)
		}
		g[0] = gfMul(g[0], root)
		root = gfMul(root, 2)
	}
	return g
}

type Encoder struct {
	roots int
	mul   [][256]byte
}

func NewEncoder(roots int) (*Encoder, error) {
	if roots < 1 || roots > 254 {
		return nil, fmt.Errorf("fec: roots must be in 1..254, got %d", roots)
	}
	g := generatorPoly(roots)
	e := &Encoder{roots: roots, mul: make([][256]byte, roots)}
	for j := 0; j < roots; j++ {
		c := g[roots-1-j]
		for f := 0; f < 256; f++ {
			e.mul[j][f] = gfMul(c, byte(f))
		}
	}
	return e, nil
}

func (e *Encoder) Roots() int { return e.roots }

func (e *Encoder) DataLen() int { return 255 - e.roots }

func (e *Encoder) Update(states, syms []byte) {
	r := e.roots
	if len(states) != len(syms)*r {
		panic(fmt.Sprintf("fec: states length %d does not match %d codewords of %d roots", len(states), len(syms), r))
	}
	if r == 2 {
		m0, m1 := &e.mul[0], &e.mul[1]
		for i, s := range syms {
			f := s ^ states[2*i]
			states[2*i] = states[2*i+1] ^ m0[f]
			states[2*i+1] = m1[f]
		}
		return
	}
	for i, s := range syms {
		st := states[i*r : i*r+r : i*r+r]
		f := s ^ st[0]
		for j := 0; j < r-1; j++ {
			st[j] = st[j+1] ^ e.mul[j][f]
		}
		st[r-1] = e.mul[r-1][f]
	}
}

func (e *Encoder) Encode(data, parity []byte) {
	r := e.roots
	if len(parity) != r {
		panic(fmt.Sprintf("fec: parity length %d does not match %d roots", len(parity), r))
	}
	clear(parity)
	for _, s := range data {
		f := s ^ parity[0]
		for j := 0; j < r-1; j++ {
			parity[j] = parity[j+1] ^ e.mul[j][f]
		}
		parity[r-1] = e.mul[r-1][f]
	}
}
