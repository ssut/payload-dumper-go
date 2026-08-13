package payload

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

func TestDebugRealDelta(t *testing.T) {
	payloadPath := os.Getenv("DBG_PAYLOAD")
	if payloadPath == "" {
		t.Skip("DBG_PAYLOAD not set")
	}
	oldDir := os.Getenv("DBG_OLD")
	targetDir := os.Getenv("DBG_TARGET")
	partName := os.Getenv("DBG_PART")

	p, err := Open(payloadPath)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var part *chromeos_update_engine.PartitionUpdate
	for _, pu := range p.manifest.GetPartitions() {
		if pu.GetPartitionName() == partName {
			part = pu
			break
		}
	}
	if part == nil {
		t.Fatalf("partition %q not found", partName)
	}

	target, err := os.ReadFile(filepath.Join(targetDir, partName+".img"))
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	src, err := openSourceImage(oldDir, part, false, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	out, err := os.CreateTemp(t.TempDir(), "dbg-*.img")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := out.Truncate(int64(part.GetNewPartitionInfo().GetSize())); err != nil {
		t.Fatal(err)
	}

	x := &partitionExtractor{p: p, name: partName, out: out, src: src, skipVerify: false}
	bs := p.blockSize
	mismatchOps := 0
	for i, op := range part.GetOperations() {
		if err := x.runOp(i, op); err != nil {
			t.Fatalf("op %d (%s) failed: %v", i, op.GetType(), err)
		}
		bad := false
		for _, e := range op.GetDstExtents() {
			off := int64(e.GetStartBlock() * bs)
			length := int64(e.GetNumBlocks() * bs)
			got := make([]byte, length)
			if _, err := out.ReadAt(got, off); err != nil {
				t.Fatal(err)
			}
			want := target[off : off+length]
			if !bytes.Equal(got, want) {
				firstDiff := int64(-1)
				for j := range got {
					if got[j] != want[j] {
						firstDiff = int64(j)
						break
					}
				}
				srcBlocks := totalBlocks(op.GetSrcExtents())
				dstBlocks := totalBlocks(op.GetDstExtents())
				fmt.Printf("MISMATCH op=%d type=%s src_extents=%d(src_blocks=%d) dst_extents=%d(dst_blocks=%d) data_len=%d extent_start=%d first_diff_at=%d got=%x want=%x\n",
					i, op.GetType(), len(op.GetSrcExtents()), srcBlocks, len(op.GetDstExtents()), dstBlocks, op.GetDataLength(), e.GetStartBlock(), firstDiff, got[firstDiff:min(firstDiff+8, length)], want[firstDiff:min(firstDiff+8, length)])
				bad = true
				break
			}
		}
		if bad {
			mismatchOps++
			if mismatchOps >= 5 {
				t.Fatalf("stopping after 5 mismatching ops")
			}
		}
	}
	if mismatchOps == 0 {
		fmt.Println("ALL OPS MATCH TARGET")
	} else {
		t.Fatalf("%d mismatching ops", mismatchOps)
	}
}
