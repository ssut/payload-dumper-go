package bsdifftest

import (
	"bytes"
	"encoding/binary"

	"github.com/andybalholm/brotli"
	dsbzip2 "github.com/dsnet/compress/bzip2"
)

type Control struct {
	DiffSize        int64
	ExtraSize       int64
	OffsetIncrement int64
}

func EncodeInt64(v int64, b []byte) {
	u := uint64(v)
	if v < 0 {
		u = uint64(-v) | 1<<63
	}
	binary.LittleEndian.PutUint64(b, u)
}

func MarshalControls(controls []Control) []byte {
	buf := make([]byte, 24*len(controls))
	for i, c := range controls {
		EncodeInt64(c.DiffSize, buf[i*24:])
		EncodeInt64(c.ExtraSize, buf[i*24+8:])
		EncodeInt64(c.OffsetIncrement, buf[i*24+16:])
	}
	return buf
}

func CompressBZ2(data []byte) []byte {
	var buf bytes.Buffer
	w, err := dsbzip2.NewWriter(&buf, nil)
	if err != nil {
		panic(err)
	}
	if _, err := w.Write(data); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func CompressBrotli(data []byte) []byte {
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func BuildBSDIFF40(controls []Control, diff, extra []byte, newSize int64) []byte {
	ctrl := CompressBZ2(MarshalControls(controls))
	diffC := CompressBZ2(diff)
	extraC := CompressBZ2(extra)
	return assemble([]byte("BSDIFF40"), ctrl, diffC, extraC, newSize)
}

func BuildBSDF2Brotli(controls []Control, diff, extra []byte, newSize int64) []byte {
	ctrl := CompressBrotli(MarshalControls(controls))
	diffC := CompressBrotli(diff)
	extraC := CompressBrotli(extra)
	return assemble([]byte{'B', 'S', 'D', 'F', '2', 2, 2, 2}, ctrl, diffC, extraC, newSize)
}

func TrivialPatch(newData []byte) []byte {
	return BuildBSDIFF40([]Control{{ExtraSize: int64(len(newData))}}, nil, newData, int64(len(newData)))
}

func TrivialPatchBrotli(newData []byte) []byte {
	return BuildBSDF2Brotli([]Control{{ExtraSize: int64(len(newData))}}, nil, newData, int64(len(newData)))
}

func assemble(magic, ctrl, diff, extra []byte, newSize int64) []byte {
	var out bytes.Buffer
	out.Write(magic)
	var num [8]byte
	EncodeInt64(int64(len(ctrl)), num[:])
	out.Write(num[:])
	EncodeInt64(int64(len(diff)), num[:])
	out.Write(num[:])
	EncodeInt64(newSize, num[:])
	out.Write(num[:])
	out.Write(ctrl)
	out.Write(diff)
	out.Write(extra)
	return out.Bytes()
}
