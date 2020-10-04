package main

import (
	"fmt"
	"os"
)

// Reader reads
type Reader struct {
	Filename string
	Offset   int64

	file      *os.File
	bytesRead int64
}

func NewReader(filename string, offset int64) *Reader {
	reader := &Reader{
		Filename: filename,
		Offset:   offset,
	}

	return reader
}

func (r *Reader) Read(p []byte) (int, error) {
	if r.file == nil {
		file, err := os.Open(r.Filename)
		if err != nil {
			return 0, err
		}

		r.file = file
		if _, err := r.file.Seek(r.Offset, 0); err != nil {
			return 0, err
		}
	}

	n, err := r.file.Read(p)
	r.bytesRead += int64(n)
	if err != nil {

		fmt.Println(err)
	}
	return n, err
}

func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}

	return nil
}
