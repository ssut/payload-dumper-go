package payload

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"google.golang.org/protobuf/proto"
)

func TestHashTreeRejectsInvalidExtents(t *testing.T) {
	for _, tc := range []struct {
		name            string
		data, tree      *chromeos_update_engine.Extent
		size, blockSize uint64
	}{
		{"tree-beyond-partition", testExtent(0, 1), testExtent(2, 1), 4096, 4096},
		{"data-beyond-partition", testExtent(2, 1), testExtent(1, 1), 8192, 4096},
		{"overlap", testExtent(0, 1), testExtent(0, 1), 8192, 4096},
		{"overflow", testExtent(0, math.MaxUint64/32), testExtent(1, 1), 8192, 4096},
		{"non-reducing-block-size", testExtent(0, 2), testExtent(2, 2), 128, 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.CreateTemp(t.TempDir(), "image")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := f.Truncate(int64(tc.size)); err != nil {
				t.Fatal(err)
			}
			part := &chromeos_update_engine.PartitionUpdate{
				PartitionName:      proto.String("test"),
				NewPartitionInfo:   &chromeos_update_engine.PartitionInfo{Size: proto.Uint64(tc.size)},
				HashTreeDataExtent: tc.data, HashTreeExtent: tc.tree, HashTreeAlgorithm: proto.String("sha256"),
			}
			if _, err := writeVerity(context.Background(), f, part, tc.blockSize, verityOptions{}, discardLogger()); err == nil {
				t.Fatal("accepted invalid hash tree")
			}
			stat, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if stat.Size() != int64(tc.size) {
				t.Fatalf("file size changed to %d", stat.Size())
			}
		})
	}
}

func TestVerityHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writeVerity(ctx, nil, &chromeos_update_engine.PartitionUpdate{}, 4096, verityOptions{}, discardLogger()); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
