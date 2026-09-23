package payload

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestE2ERealPayload(t *testing.T) {
	payloadPath := os.Getenv("E2E_PAYLOAD")
	if payloadPath == "" {
		t.Skip("E2E_PAYLOAD not set")
	}
	oldDir := os.Getenv("E2E_OLD")
	targetDir := os.Getenv("E2E_TARGET")
	outDir := os.Getenv("E2E_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}

	p, err := Open(payloadPath)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	p.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	var fecParts []string
	for _, part := range p.manifest.GetPartitions() {
		fecExt := part.GetFecExtent()
		if fecExt.GetNumBlocks() == 0 {
			continue
		}
		fecParts = append(fecParts, fmt.Sprintf("%s(roots=%d data_blocks=%d fec_blocks=%d)",
			part.GetPartitionName(), part.GetFecRoots(),
			part.GetFecDataExtent().GetNumBlocks(), fecExt.GetNumBlocks()))
	}
	if len(fecParts) == 0 {
		t.Log("WARNING: this payload declares no FEC extents; the run does not exercise FEC generation")
	} else {
		t.Logf("fec partitions: %s", strings.Join(fecParts, ", "))
	}

	var selected []string
	if v := os.Getenv("E2E_PARTS"); v != "" {
		selected = strings.Split(v, ",")
	}

	err = p.Extract(context.Background(), ExtractOptions{
		OutputDir:  outDir,
		Partitions: selected,
		SourceDir:  oldDir,
	})
	if err == nil {
		t.Logf("PASS: every extracted image matches new_partition_info.hash (output in %s)", outDir)
		return
	}
	t.Errorf("extraction failed: %v", err)

	if targetDir == "" {
		return
	}
	for _, part := range p.manifest.GetPartitions() {
		name := part.GetPartitionName()
		if len(selected) > 0 && !slices.Contains(selected, name) {
			continue
		}
		got, readErr := os.ReadFile(filepath.Join(outDir, name+".img"))
		if readErr != nil {
			continue
		}
		want, readErr := os.ReadFile(filepath.Join(targetDir, name+".img"))
		if readErr != nil {
			continue
		}
		if bytes.Equal(got, want) {
			continue
		}
		off := firstDiff(got, want)
		t.Errorf("%s: first difference at byte %d (%s)", name, off, classifyOffset(part, uint64(off), p.blockSize))
	}
}

func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func classifyOffset(part *chromeos_update_engine.PartitionUpdate, off, blockSize uint64) string {
	blk := off / blockSize
	within := func(e *chromeos_update_engine.Extent) bool {
		return e.GetNumBlocks() > 0 && blk >= e.GetStartBlock() && blk < e.GetStartBlock()+e.GetNumBlocks()
	}
	switch {
	case within(part.GetFecExtent()):
		return "inside the FEC parity extent"
	case within(part.GetHashTreeExtent()):
		return "inside the hash tree extent"
	}
	for _, op := range part.GetOperations() {
		for _, e := range op.GetDstExtents() {
			if within(e) {
				return "inside a region written by install operations"
			}
		}
	}
	return "inside a region carried over from the source image"
}
