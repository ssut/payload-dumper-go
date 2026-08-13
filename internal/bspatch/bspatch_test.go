package bspatch

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ssut/payload-dumper-go/internal/bspatch/bsdifftest"
)

func TestApplyExtraOnly(t *testing.T) {
	want := []byte("hello incremental world")
	patch := bsdifftest.TrivialPatch(want)
	got, err := Apply([]byte("completely unrelated old data"), patch, int64(len(want)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyDiffAdd(t *testing.T) {
	old := []byte{10, 20, 30, 250}
	diff := []byte{1, 2, 3, 10}
	want := []byte{11, 22, 33, 4}
	patch := bsdifftest.BuildBSDIFF40([]bsdifftest.Control{{DiffSize: 4}}, diff, nil, 4)
	got, err := Apply(old, patch, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestApplyNegativeSeek(t *testing.T) {
	old := []byte{1, 2, 3, 4}
	controls := []bsdifftest.Control{
		{DiffSize: 2, OffsetIncrement: -2},
		{DiffSize: 2},
	}
	diff := []byte{0, 0, 100, 100}
	want := []byte{1, 2, 101, 102}
	patch := bsdifftest.BuildBSDIFF40(controls, diff, nil, 4)
	got, err := Apply(old, patch, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestApplyOldOutOfBoundsReadsZero(t *testing.T) {
	old := []byte{5, 5}
	diff := []byte{1, 1, 7, 8}
	want := []byte{6, 6, 7, 8}
	patch := bsdifftest.BuildBSDIFF40([]bsdifftest.Control{{DiffSize: 4}}, diff, nil, 4)
	got, err := Apply(old, patch, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestApplyBSDF2Brotli(t *testing.T) {
	old := []byte{100, 100, 100}
	controls := []bsdifftest.Control{{DiffSize: 3, ExtraSize: 2}}
	diff := []byte{1, 2, 3}
	extra := []byte{9, 9}
	want := []byte{101, 102, 103, 9, 9}
	patch := bsdifftest.BuildBSDF2Brotli(controls, diff, extra, 5)
	got, err := Apply(old, patch, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestApplyRejectsBadMagic(t *testing.T) {
	patch := bytes.Repeat([]byte{0}, 40)
	if _, err := Apply(nil, patch, 100); err == nil || !strings.Contains(err.Error(), "magic") {
		t.Fatalf("expected magic error, got %v", err)
	}
}

func TestApplyRejectsShortPatch(t *testing.T) {
	if _, err := Apply(nil, []byte("BSDIFF40"), 100); err == nil {
		t.Fatal("expected error for short patch")
	}
}

func TestApplyRejectsOversizedOutput(t *testing.T) {
	patch := bsdifftest.TrivialPatch([]byte("0123456789"))
	if _, err := Apply(nil, patch, 5); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("expected capacity error, got %v", err)
	}
}

func TestApplyRejectsDiffPastEnd(t *testing.T) {
	patch := bsdifftest.BuildBSDIFF40([]bsdifftest.Control{{DiffSize: 8}}, make([]byte, 8), nil, 4)
	if _, err := Apply(nil, patch, 4); err == nil {
		t.Fatal("expected error for diff past end")
	}
}

func TestApplyRejectsNegativeControl(t *testing.T) {
	patch := bsdifftest.BuildBSDIFF40([]bsdifftest.Control{{DiffSize: -1}}, nil, nil, 4)
	if _, err := Apply(nil, patch, 4); err == nil {
		t.Fatal("expected error for negative control size")
	}
}

func TestApplyUnsupportedCompressor(t *testing.T) {
	patch := bsdifftest.TrivialPatchBrotli([]byte("data"))
	patch[5] = 0
	if _, err := Apply(nil, patch, 4); err == nil || !strings.Contains(err.Error(), "compressor") {
		t.Fatalf("expected compressor error, got %v", err)
	}
}

func TestParseInt64(t *testing.T) {
	cases := []struct {
		value int64
	}{
		{0}, {1}, {-1}, {4096}, {-4096}, {1<<62 + 12345}, {-(1<<62 + 12345)},
	}
	for _, tc := range cases {
		var buf [8]byte
		bsdifftest.EncodeInt64(tc.value, buf[:])
		if got := parseInt64(buf[:]); got != tc.value {
			t.Errorf("roundtrip %d: got %d", tc.value, got)
		}
	}
	negativeZero := [8]byte{0, 0, 0, 0, 0, 0, 0, 0x80}
	if got := parseInt64(negativeZero[:]); got != 0 {
		t.Errorf("negative zero decoded to %d, want 0", got)
	}
}
