package payload

import (
	"bytes"
	"io"
	"math"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func testExtent(start, num uint64) *chromeos_update_engine.Extent {
	return &chromeos_update_engine.Extent{StartBlock: proto.Uint64(start), NumBlocks: proto.Uint64(num)}
}

type writeAtBuffer struct {
	buf []byte
}

func (w *writeAtBuffer) WriteAt(p []byte, off int64) (int, error) {
	if int(off)+len(p) > len(w.buf) {
		return 0, io.ErrShortWrite
	}
	copy(w.buf[off:], p)
	return len(p), nil
}

func TestExtentsRoundTrip(t *testing.T) {
	const bs = 4
	src := []byte("aaaabbbbccccdddd")
	extents := []*chromeos_update_engine.Extent{testExtent(3, 1), testExtent(0, 2)}
	r, err := newExtentsReader(bytes.NewReader(src), extents, bs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := "ddddaaaabbbb"; string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	out := &writeAtBuffer{buf: make([]byte, 16)}
	w, err := newExtentsWriter(out, extents, bs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("DDDDAAAABBBB")); err != nil {
		t.Fatal(err)
	}
	if want := "AAAABBBB\x00\x00\x00\x00DDDD"; string(out.buf) != want {
		t.Fatalf("got %q, want %q", out.buf, want)
	}
}

func TestExtentsWriterRejectsOverflowWrite(t *testing.T) {
	out := &writeAtBuffer{buf: make([]byte, 16)}
	w, err := newExtentsWriter(out, []*chromeos_update_engine.Extent{testExtent(0, 1)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("12345")); err == nil {
		t.Fatal("expected error when writing past extents")
	}
}

func TestExtentsReaderShortSource(t *testing.T) {
	r, err := newExtentsReader(bytes.NewReader([]byte("ab")), []*chromeos_update_engine.Extent{testExtent(0, 1)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(r); err == nil {
		t.Fatal("expected error when source is shorter than extents")
	}
}

func TestValidateExtentsRejectsSparseHole(t *testing.T) {
	err := validateExtents([]*chromeos_update_engine.Extent{testExtent(math.MaxUint64, 1)}, 4096)
	if err == nil {
		t.Fatal("expected sparse hole rejection")
	}
}
