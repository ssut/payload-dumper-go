// Package fectest is a naive, spec-literal reference implementation of the
// dm-verity FEC encoding, used as a test oracle for package fec.
//
// It is deliberately derived differently from the production code so that
// agreement between the two is a real cross-check rather than an echo: field
// arithmetic here goes through log/exp tables (production builds per-tap
// product tables with carry-less multiplication), and the block interleave
// uses the upstream fec_ecc_interleave formula verbatim (production uses the
// algebraically simplified form).
//
// It favors obviousness over speed and panics on invalid input. Nothing
// outside tests should import it.
package fectest

import "fmt"

// BlockSize is FEC_BLOCKSIZE from AOSP libfec, which hardcodes 4096.
const BlockSize = 4096

var (
	expTab [512]byte
	logTab [256]byte
)

func init() {
	x := byte(1)
	for i := 0; i < 255; i++ {
		expTab[i] = x
		logTab[x] = byte(i)
		hi := x & 0x80
		x <<= 1
		if hi != 0 {
			x ^= 0x1d
		}
	}
	for i := 255; i < 512; i++ {
		expTab[i] = expTab[i-255]
	}
}

func Mul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return expTab[int(logTab[a])+int(logTab[b])]
}

func Exp(n int) byte {
	return expTab[((n%255)+255)%255]
}

func Gen(roots int) []byte {
	if roots < 1 || roots > 254 {
		panic(fmt.Sprintf("fectest: roots out of range: %d", roots))
	}
	g := []byte{1}
	for i := 0; i < roots; i++ {
		root := Exp(i)
		next := make([]byte, len(g)+1)
		for j, c := range g {
			next[j] ^= c
			next[j+1] ^= Mul(c, root)
		}
		g = next
	}
	return g
}

func Encode(msg []byte, roots int) []byte {
	if len(msg) > 255-roots {
		panic(fmt.Sprintf("fectest: message of %d symbols exceeds %d", len(msg), 255-roots))
	}
	g := Gen(roots)
	buf := make([]byte, len(msg)+roots)
	copy(buf, msg)
	for i := 0; i < len(msg); i++ {
		c := buf[i]
		if c == 0 {
			continue
		}
		for j := 1; j <= roots; j++ {
			buf[i+j] ^= Mul(g[j], c)
		}
	}
	return buf[len(msg):]
}

func Eval(poly []byte, x byte) byte {
	var acc byte
	for _, c := range poly {
		acc = Mul(acc, x) ^ c
	}
	return acc
}

func Interleave(offset int64, rsn int, rounds int64) int64 {
	return offset/int64(rsn) + (offset%int64(rsn))*rounds*BlockSize
}

func Rounds(dataBlocks, roots int) int {
	rsn := 255 - roots
	return (dataBlocks + rsn - 1) / rsn
}

func FEC(data []byte, roots int) []byte {
	if len(data)%BlockSize != 0 {
		panic(fmt.Sprintf("fectest: data length %d is not a multiple of %d", len(data), BlockSize))
	}
	if roots < 1 || roots > 254 {
		panic(fmt.Sprintf("fectest: roots out of range: %d", roots))
	}
	rsn := 255 - roots
	dataBlocks := len(data) / BlockSize
	rounds := Rounds(dataBlocks, roots)
	out := make([]byte, 0, rounds*roots*BlockSize)
	msg := make([]byte, rsn)
	for i := 0; i < rounds; i++ {
		for k := 0; k < BlockSize; k++ {
			for j := 0; j < rsn; j++ {
				block := Interleave(int64(i)*int64(rsn)*BlockSize+int64(j), rsn, int64(rounds))
				idx := block + int64(k)
				if idx < int64(len(data)) {
					msg[j] = data[idx]
				} else {
					msg[j] = 0
				}
			}
			out = append(out, Encode(msg, roots)...)
		}
	}
	return out
}
