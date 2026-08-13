package payload

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

type sourceImage struct {
	path string
	f    *os.File
	size int64
}

func openSourceImage(dir string, part *chromeos_update_engine.PartitionUpdate, skipVerify bool, logger *slog.Logger) (*sourceImage, error) {
	name := part.GetPartitionName()
	path := filepath.Join(dir, name+".img")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("payload: source image for partition %q not available at %s: %w", name, path, err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	src := &sourceImage{path: path, f: f, size: st.Size()}
	oldInfo := part.GetOldPartitionInfo()
	if oldSize := oldInfo.GetSize(); oldSize > 0 && st.Size() < int64(oldSize) {
		f.Close()
		return nil, fmt.Errorf("payload: source image %s is smaller than expected (%d < %d bytes); wrong or truncated source for partition %q", path, st.Size(), oldSize, name)
	}
	if skipVerify {
		return src, nil
	}
	if needsFullSourceVerification(part) && len(oldInfo.GetHash()) > 0 {
		logger.Debug("verifying source image hash", slog.String("partition", name), slog.String("path", path))
		h := sha256.New()
		if _, err := io.Copy(h, io.NewSectionReader(f, 0, int64(oldInfo.GetSize()))); err != nil {
			f.Close()
			return nil, fmt.Errorf("payload: hashing source image %s: %w", path, err)
		}
		if sum := h.Sum(nil); !hashEqual(sum, oldInfo.GetHash()) {
			f.Close()
			return nil, &VerificationError{
				Partition: name,
				Subject:   "source image",
				Expected:  hex.EncodeToString(oldInfo.GetHash()),
				Actual:    hex.EncodeToString(sum),
			}
		}
	}
	return src, nil
}

func (s *sourceImage) Close() error {
	if s == nil || s.f == nil {
		return nil
	}
	return s.f.Close()
}

func needsFullSourceVerification(part *chromeos_update_engine.PartitionUpdate) bool {
	for _, op := range part.GetOperations() {
		if operationNeedsSource(op) && len(op.GetSrcSha256Hash()) == 0 {
			return true
		}
	}
	return false
}

func hashEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
