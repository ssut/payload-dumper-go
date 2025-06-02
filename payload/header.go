package payload

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Header represents a payload file header
type Header struct {
	Version              uint64
	ManifestLen          uint64
	MetadataSignatureLen uint32
	Size                 uint64
}

// ReadHeader reads and parses the payload header from a reader
func ReadHeader(r io.Reader) (*Header, error) {
	header := &Header{}

	// Read magic bytes
	buf := make([]byte, 4)
	if _, err := r.Read(buf); err != nil {
		return nil, err
	}
	if string(buf) != PayloadHeaderMagic {
		return nil, fmt.Errorf("invalid payload magic: %s", buf)
	}

	// Read Version
	buf = make([]byte, 8)
	if _, err := r.Read(buf); err != nil {
		return nil, err
	}
	header.Version = binary.BigEndian.Uint64(buf)

	if header.Version != BrilloMajorPayloadVersion {
		return nil, fmt.Errorf("unsupported payload version: %d", header.Version)
	}

	// Read Manifest Length
	buf = make([]byte, 8)
	if _, err := r.Read(buf); err != nil {
		return nil, err
	}
	header.ManifestLen = binary.BigEndian.Uint64(buf)

	header.Size = 24

	// Read Metadata Signature Length
	buf = make([]byte, 4)
	if _, err := r.Read(buf); err != nil {
		return nil, err
	}
	header.MetadataSignatureLen = binary.BigEndian.Uint32(buf)

	return header, nil
}