package payload

import (
	"compress/bzip2"
	"io"

	xz "github.com/spencercw/go-xz"
	"github.com/valyala/gozstd"
)

type readCloser struct {
	io.Reader
	closeFn func() error
}

func (rc readCloser) Close() error {
	if rc.closeFn != nil {
		return rc.closeFn()
	}
	return nil
}

func newXZReader(r io.Reader) io.ReadCloser {
	d := xz.NewDecompressionReader(r)
	return readCloser{Reader: &d, closeFn: func() error {
		d.Close()
		return nil
	}}
}

func newBzip2Reader(r io.Reader) io.ReadCloser {
	return readCloser{Reader: bzip2.NewReader(r)}
}

func newZstdReader(r io.Reader) io.ReadCloser {
	zr := gozstd.NewReader(r)
	return readCloser{Reader: zr, closeFn: func() error {
		zr.Release()
		return nil
	}}
}
