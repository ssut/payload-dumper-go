package payload_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"github.com/ssut/payload-dumper-go/payload"
	"google.golang.org/protobuf/proto"
)

func TestExtractRejectsHashTreeBeyondVerifiedImage(t *testing.T) {
	data := block(7)
	raw := buildPayloadWith(t, 0, []partitionSpec{{name: "system", newData: data, ops: []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: data, dst: []*chromeos_update_engine.Extent{ext(0, 1)}}}}}, func(part *chromeos_update_engine.PartitionUpdate) {
		part.HashTreeDataExtent = ext(0, 1)
		part.HashTreeExtent = ext(2, 1)
		part.HashTreeAlgorithm = proto.String("sha256")
	})
	p := openPayload(t, raw)
	output := t.TempDir()
	if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: output}); err == nil {
		t.Fatal("accepted hash tree extending beyond the verified image")
	}
	if _, err := os.Stat(filepath.Join(output, "system.img")); !os.IsNotExist(err) {
		t.Fatalf("incomplete output retained: %v", err)
	}
}
