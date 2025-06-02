package payload

import (
	"os"
)

// Reader provides a custom reader implementation for payload files
type Reader struct {
	Filename string
	Offset   int64

	file      *os.File
	bytesRead int64
}

// NewReader creates a new Reader instance
func NewReader(filename string, offset int64) *Reader {
	return &Reader{
		Filename: filename,
		Offset:   offset,
	}
}

// Read implements io.Reader interface
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
	return n, err
}

// Close closes the underlying file
func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// BytesRead returns the number of bytes read so far
func (r *Reader) BytesRead() int64 {
	return r.bytesRead
}