package payload

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	headerMagic        = "CrAU"
	majorVersionBrillo = 2
	headerLen          = 24
)

type Header struct {
	Version              uint64
	ManifestLen          uint64
	MetadataSignatureLen uint32
}

func readHeader(r io.Reader) (Header, error) {
	var h Header
	buf := make([]byte, headerLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return h, fmt.Errorf("payload: reading header: %w", err)
	}
	if string(buf[0:4]) != headerMagic {
		return h, ErrNotPayload
	}
	h.Version = binary.BigEndian.Uint64(buf[4:12])
	if h.Version != majorVersionBrillo {
		return h, fmt.Errorf("payload: unsupported payload major version %d (only version %d is supported)", h.Version, majorVersionBrillo)
	}
	h.ManifestLen = binary.BigEndian.Uint64(buf[12:20])
	if h.ManifestLen == 0 {
		return h, fmt.Errorf("payload: manifest length is zero")
	}
	h.MetadataSignatureLen = binary.BigEndian.Uint32(buf[20:24])
	return h, nil
}
