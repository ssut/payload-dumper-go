package payload

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
)

func Open(name string) (*Payload, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	var magic [4]byte
	if _, err := f.ReadAt(magic[:], 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("payload: reading %s: %w", name, err)
	}
	switch {
	case string(magic[:]) == headerMagic:
		p, err := NewFromReaderAt(f, st.Size())
		if err != nil {
			f.Close()
			return nil, err
		}
		p.closers = append(p.closers, f)
		return p, nil
	case magic[0] == 'P' && magic[1] == 'K':
		return openFromZip(f, st.Size())
	default:
		f.Close()
		return nil, ErrNotPayload
	}
}

func openFromZip(f *os.File, size int64) (*Payload, error) {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("payload: opening zip archive: %w", err)
	}
	entry := findPayloadEntry(zr)
	if entry == nil {
		f.Close()
		return nil, ErrNotPayload
	}
	if entry.Method == zip.Store {
		offset, err := entry.DataOffset()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("payload: locating payload.bin in zip: %w", err)
		}
		length := int64(entry.UncompressedSize64)
		p, err := NewFromReaderAt(io.NewSectionReader(f, offset, length), length)
		if err != nil {
			f.Close()
			return nil, err
		}
		p.closers = append(p.closers, f)
		return p, nil
	}
	return extractZipEntryToTemp(f, entry)
}

func findPayloadEntry(zr *zip.Reader) *zip.File {
	var fallback *zip.File
	for _, zf := range zr.File {
		if zf.UncompressedSize64 == 0 {
			continue
		}
		if zf.Name == "payload.bin" {
			return zf
		}
		if fallback == nil && path.Base(zf.Name) == "payload.bin" {
			fallback = zf
		}
	}
	return fallback
}

func extractZipEntryToTemp(f *os.File, entry *zip.File) (*Payload, error) {
	defer f.Close()
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("payload: opening payload.bin in zip: %w", err)
	}
	defer rc.Close()
	tmp, err := os.CreateTemp("", "payload_*.bin")
	if err != nil {
		return nil, fmt.Errorf("payload: creating temp file: %w", err)
	}
	cleanup := func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}
	n, err := io.Copy(tmp, rc)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("payload: extracting payload.bin from zip: %w", err)
	}
	p, err := NewFromReaderAt(tmp, n)
	if err != nil {
		cleanup()
		return nil, err
	}
	p.closers = append(p.closers, tmp)
	p.tempPath = tmp.Name()
	return p, nil
}
