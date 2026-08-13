package payload_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	xz "github.com/spencercw/go-xz"
	"github.com/valyala/gozstd"
	"google.golang.org/protobuf/proto"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"github.com/ssut/payload-dumper-go/internal/bspatch/bsdifftest"
	"github.com/ssut/payload-dumper-go/payload"
)

const blockSize = 4096

func block(b byte) []byte {
	return bytes.Repeat([]byte{b}, blockSize)
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func ext(start, num uint64) *chromeos_update_engine.Extent {
	return &chromeos_update_engine.Extent{StartBlock: proto.Uint64(start), NumBlocks: proto.Uint64(num)}
}

func sum(data []byte) []byte {
	s := sha256.Sum256(data)
	return s[:]
}

func bz2(t *testing.T, data []byte) []byte {
	t.Helper()
	return bsdifftest.CompressBZ2(data)
}

func xzCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := xz.NewCompressionWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type opSpec struct {
	typ     chromeos_update_engine.InstallOperation_Type
	data    []byte
	src     []*chromeos_update_engine.Extent
	dst     []*chromeos_update_engine.Extent
	srcHash []byte
	noHash  bool
}

type partitionSpec struct {
	name      string
	newData   []byte
	oldData   []byte
	ops       []opSpec
	badHash   bool
	noNewHash bool
}

func buildPayload(t *testing.T, minor uint32, parts []partitionSpec) []byte {
	t.Helper()
	return buildPayloadWith(t, minor, parts, nil)
}

func buildPayloadWith(t *testing.T, minor uint32, parts []partitionSpec, mutate func(*chromeos_update_engine.PartitionUpdate)) []byte {
	t.Helper()
	var blobs bytes.Buffer
	var updates []*chromeos_update_engine.PartitionUpdate
	for _, ps := range parts {
		var ops []*chromeos_update_engine.InstallOperation
		for _, o := range ps.ops {
			op := &chromeos_update_engine.InstallOperation{
				Type:       o.typ.Enum(),
				SrcExtents: o.src,
				DstExtents: o.dst,
			}
			if o.data != nil {
				op.DataOffset = proto.Uint64(uint64(blobs.Len()))
				op.DataLength = proto.Uint64(uint64(len(o.data)))
				if !o.noHash {
					op.DataSha256Hash = sum(o.data)
				}
				blobs.Write(o.data)
			}
			if o.srcHash != nil {
				op.SrcSha256Hash = o.srcHash
			}
			ops = append(ops, op)
		}
		update := &chromeos_update_engine.PartitionUpdate{
			PartitionName: proto.String(ps.name),
			Operations:    ops,
			NewPartitionInfo: &chromeos_update_engine.PartitionInfo{
				Size: proto.Uint64(uint64(len(ps.newData))),
				Hash: sum(ps.newData),
			},
		}
		if ps.badHash {
			update.NewPartitionInfo.Hash = sum([]byte("wrong"))
		}
		if ps.noNewHash {
			update.NewPartitionInfo.Hash = nil
		}
		if ps.oldData != nil {
			update.OldPartitionInfo = &chromeos_update_engine.PartitionInfo{
				Size: proto.Uint64(uint64(len(ps.oldData))),
				Hash: sum(ps.oldData),
			}
		}
		if mutate != nil {
			mutate(update)
		}
		updates = append(updates, update)
	}
	manifest := &chromeos_update_engine.DeltaArchiveManifest{
		BlockSize:    proto.Uint32(blockSize),
		MinorVersion: proto.Uint32(minor),
		Partitions:   updates,
	}
	manifestBytes, err := proto.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	out.WriteString("CrAU")
	binary.Write(&out, binary.BigEndian, uint64(2))
	binary.Write(&out, binary.BigEndian, uint64(len(manifestBytes)))
	binary.Write(&out, binary.BigEndian, uint32(0))
	out.Write(manifestBytes)
	out.Write(blobs.Bytes())
	return out.Bytes()
}

func writePayloadFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func openPayload(t *testing.T, data []byte) *payload.Payload {
	t.Helper()
	p, err := payload.Open(writePayloadFile(t, data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func readImage(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name+".img"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExtractFullPayload(t *testing.T) {
	bootNew := concat(block('A'), block('B'), make([]byte, blockSize), block('C'), block('D'), make([]byte, blockSize))
	vendorNew := block('E')
	data := buildPayload(t, 0, []partitionSpec{
		{
			name:    "boot",
			newData: bootNew,
			ops: []opSpec{
				{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('A'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}},
				{typ: chromeos_update_engine.InstallOperation_REPLACE_BZ, data: bz2(t, block('B')), dst: []*chromeos_update_engine.Extent{ext(1, 1)}},
				{typ: chromeos_update_engine.InstallOperation_ZERO, dst: []*chromeos_update_engine.Extent{ext(2, 1), ext(5, 1)}},
				{typ: chromeos_update_engine.InstallOperation_REPLACE_XZ, data: xzCompress(t, concat(block('C'), block('D'))), dst: []*chromeos_update_engine.Extent{ext(3, 2)}},
			},
		},
		{
			name:    "vendor",
			newData: vendorNew,
			ops: []opSpec{
				{typ: chromeos_update_engine.InstallOperation_ZSTD, data: gozstd.Compress(nil, block('E')), dst: []*chromeos_update_engine.Extent{ext(0, 1)}},
			},
		},
	})
	p := openPayload(t, data)
	if p.IsDelta() {
		t.Fatal("full payload misdetected as delta")
	}
	parts := p.Partitions()
	if len(parts) != 2 || parts[0].Name != "boot" || parts[0].Size != uint64(len(bootNew)) {
		t.Fatalf("unexpected partitions: %+v", parts)
	}

	var mu sync.Mutex
	doneEvents := map[string]payload.ProgressEvent{}
	outDir := t.TempDir()
	err := p.Extract(context.Background(), payload.ExtractOptions{
		OutputDir: outDir,
		OnProgress: func(ev payload.ProgressEvent) {
			mu.Lock()
			defer mu.Unlock()
			if ev.Done {
				doneEvents[ev.Partition] = ev
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readImage(t, outDir, "boot"); !bytes.Equal(got, bootNew) {
		t.Fatal("boot image mismatch")
	}
	if got := readImage(t, outDir, "vendor"); !bytes.Equal(got, vendorNew) {
		t.Fatal("vendor image mismatch")
	}
	for _, name := range []string{"boot", "vendor"} {
		ev, ok := doneEvents[name]
		if !ok || ev.Err != nil || ev.CompletedOps != ev.TotalOps {
			t.Fatalf("missing or failed done event for %s: %+v", name, ev)
		}
	}
}

func TestExtractSelectedPartitions(t *testing.T) {
	data := buildPayload(t, 0, []partitionSpec{
		{name: "boot", newData: block('A'), ops: []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('A'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}}}},
		{name: "vendor", newData: block('B'), ops: []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('B'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}}}},
	})
	p := openPayload(t, data)
	outDir := t.TempDir()
	if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, Partitions: []string{"vendor"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "boot.img")); !os.IsNotExist(err) {
		t.Fatal("boot.img should not have been extracted")
	}
	if got := readImage(t, outDir, "vendor"); !bytes.Equal(got, block('B')) {
		t.Fatal("vendor image mismatch")
	}
}

func TestExtractUnknownPartition(t *testing.T) {
	data := buildPayload(t, 0, []partitionSpec{
		{name: "boot", newData: block('A'), ops: []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('A'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}}}},
	})
	p := openPayload(t, data)
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: t.TempDir(), Partitions: []string{"boot", "nonexistent"}})
	var unknown *payload.UnknownPartitionsError
	if !errors.As(err, &unknown) || unknown.Names[0] != "nonexistent" {
		t.Fatalf("expected UnknownPartitionsError, got %v", err)
	}
}

func deltaPayloadSpec(t *testing.T, oldBoot []byte) ([]byte, []byte) {
	target := block('Q')
	diff := make([]byte, blockSize)
	for i := range diff {
		diff[i] = target[i] - oldBoot[i]
	}
	newBoot := concat(block('z'), target, block('R'))
	data := buildPayload(t, 8, []partitionSpec{
		{
			name:    "boot",
			newData: newBoot,
			oldData: oldBoot,
			ops: []opSpec{
				{
					typ:     chromeos_update_engine.InstallOperation_SOURCE_COPY,
					src:     []*chromeos_update_engine.Extent{ext(2, 1)},
					dst:     []*chromeos_update_engine.Extent{ext(0, 1)},
					srcHash: sum(oldBoot[2*blockSize : 3*blockSize]),
				},
				{
					typ:     chromeos_update_engine.InstallOperation_SOURCE_BSDIFF,
					data:    bsdifftest.BuildBSDIFF40([]bsdifftest.Control{{DiffSize: blockSize}}, diff, nil, blockSize),
					src:     []*chromeos_update_engine.Extent{ext(0, 1)},
					dst:     []*chromeos_update_engine.Extent{ext(1, 1)},
					srcHash: sum(oldBoot[0:blockSize]),
				},
				{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('R'), dst: []*chromeos_update_engine.Extent{ext(2, 1)}},
			},
		},
		{
			name:    "modem",
			newData: block('S'),
			oldData: block('m'),
			ops: []opSpec{
				{
					typ:     chromeos_update_engine.InstallOperation_BROTLI_BSDIFF,
					data:    bsdifftest.TrivialPatchBrotli(block('S')),
					src:     []*chromeos_update_engine.Extent{ext(0, 1)},
					dst:     []*chromeos_update_engine.Extent{ext(0, 1)},
					srcHash: sum(block('m')),
				},
			},
		},
	})
	return data, newBoot
}

func writeSourceDir(t *testing.T, images map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range images {
		if err := os.WriteFile(filepath.Join(dir, name+".img"), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestExtractDeltaPayload(t *testing.T) {
	oldBoot := concat(block('x'), block('y'), block('z'))
	data, newBoot := deltaPayloadSpec(t, oldBoot)
	p := openPayload(t, data)
	if !p.IsDelta() {
		t.Fatal("delta payload not detected")
	}
	srcDir := writeSourceDir(t, map[string][]byte{"boot": oldBoot, "modem": block('m')})
	outDir := t.TempDir()
	if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, SourceDir: srcDir}); err != nil {
		t.Fatal(err)
	}
	if got := readImage(t, outDir, "boot"); !bytes.Equal(got, newBoot) {
		t.Fatal("boot image mismatch after delta apply")
	}
	if got := readImage(t, outDir, "modem"); !bytes.Equal(got, block('S')) {
		t.Fatal("modem image mismatch after delta apply")
	}
}

func TestExtractDeltaWithoutSourceDir(t *testing.T) {
	oldBoot := concat(block('x'), block('y'), block('z'))
	data, _ := deltaPayloadSpec(t, oldBoot)
	p := openPayload(t, data)
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: t.TempDir()})
	var missing *payload.MissingSourceError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingSourceError, got %v", err)
	}
	if len(missing.Partitions) != 2 {
		t.Fatalf("expected both partitions listed, got %v", missing.Partitions)
	}
}

func TestExtractDeltaWrongSource(t *testing.T) {
	oldBoot := concat(block('x'), block('y'), block('z'))
	data, _ := deltaPayloadSpec(t, oldBoot)
	p := openPayload(t, data)
	wrongOld := concat(block('1'), block('2'), block('3'))
	srcDir := writeSourceDir(t, map[string][]byte{"boot": wrongOld, "modem": block('m')})
	outDir := t.TempDir()
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, SourceDir: srcDir, Partitions: []string{"boot"}})
	var verification *payload.VerificationError
	if !errors.As(err, &verification) {
		t.Fatalf("expected VerificationError, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outDir, "boot.img")); !os.IsNotExist(statErr) {
		t.Fatal("corrupt boot.img should have been removed")
	}
}

func TestExtractDeltaMissingSourceImage(t *testing.T) {
	oldBoot := concat(block('x'), block('y'), block('z'))
	data, _ := deltaPayloadSpec(t, oldBoot)
	p := openPayload(t, data)
	srcDir := writeSourceDir(t, map[string][]byte{"boot": oldBoot})
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: t.TempDir(), SourceDir: srcDir})
	if err == nil || !strings.Contains(err.Error(), "modem") {
		t.Fatalf("expected error mentioning missing modem source, got %v", err)
	}
}

func TestExtractUnsupportedOperation(t *testing.T) {
	data := buildPayload(t, 8, []partitionSpec{
		{
			name:    "system",
			newData: block('A'),
			oldData: block('o'),
			ops: []opSpec{
				{typ: chromeos_update_engine.InstallOperation_PUFFDIFF, data: []byte("PUF1junk"), src: []*chromeos_update_engine.Extent{ext(0, 1)}, dst: []*chromeos_update_engine.Extent{ext(0, 1)}},
			},
		},
	})
	p := openPayload(t, data)
	outDir := t.TempDir()
	srcDir := writeSourceDir(t, map[string][]byte{"system": block('o')})
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, SourceDir: srcDir})
	var unsupported *payload.UnsupportedOperationsError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected UnsupportedOperationsError, got %v", err)
	}
	if ops := unsupported.Partitions["system"]; len(ops) != 1 || ops[0] != "PUFFDIFF" {
		t.Fatalf("unexpected unsupported detail: %v", unsupported.Partitions)
	}
	if _, statErr := os.Stat(filepath.Join(outDir, "system.img")); !os.IsNotExist(statErr) {
		t.Fatal("no output should be written when operations are unsupported")
	}
}

func TestExtractOutputHashMismatch(t *testing.T) {
	data := buildPayload(t, 0, []partitionSpec{
		{
			name:    "boot",
			newData: block('A'),
			badHash: true,
			ops:     []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('A'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}}},
		},
	})
	p := openPayload(t, data)
	outDir := t.TempDir()
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir})
	var verification *payload.VerificationError
	if !errors.As(err, &verification) || verification.Subject != "output image" {
		t.Fatalf("expected output image VerificationError, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outDir, "boot.img")); !os.IsNotExist(statErr) {
		t.Fatal("mismatched boot.img should have been removed")
	}
	if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, SkipVerify: true}); err != nil {
		t.Fatalf("SkipVerify should bypass output hash check, got %v", err)
	}
}

func TestOpenFromZip(t *testing.T) {
	payloadBytes := buildPayload(t, 0, []partitionSpec{
		{name: "boot", newData: block('A'), ops: []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('A'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}}}},
	})
	for _, tc := range []struct {
		name   string
		method uint16
	}{
		{"stored", zip.Store},
		{"deflated", zip.Deflate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zipPath := filepath.Join(t.TempDir(), "ota.zip")
			f, err := os.Create(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			zw := zip.NewWriter(f)
			for _, entry := range []struct {
				name string
				data []byte
			}{
				{"META-INF/com/android/metadata", []byte("ota-type=AB\n")},
				{"payload.bin", payloadBytes},
			} {
				w, err := zw.CreateHeader(&zip.FileHeader{Name: entry.name, Method: tc.method})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write(entry.data); err != nil {
					t.Fatal(err)
				}
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			p, err := payload.Open(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			outDir := t.TempDir()
			if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir}); err != nil {
				t.Fatal(err)
			}
			if got := readImage(t, outDir, "boot"); !bytes.Equal(got, block('A')) {
				t.Fatal("boot image mismatch from zip")
			}
		})
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junk.bin")
	if err := os.WriteFile(path, []byte("this is definitely not a payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Open(path); !errors.Is(err, payload.ErrNotPayload) {
		t.Fatalf("expected ErrNotPayload, got %v", err)
	}
}

func TestOpenRejectsZipWithoutPayload(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "no-payload.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("something.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Open(zipPath); !errors.Is(err, payload.ErrNotPayload) {
		t.Fatalf("expected ErrNotPayload, got %v", err)
	}
}

func TestExtractDeltaFallthroughGaps(t *testing.T) {
	oldBoot := concat(block('a'), block('b'), block('c'), block('d'))
	newBoot := concat(block('N'), block('b'), block('a'), block('d'))
	data := buildPayload(t, 9, []partitionSpec{
		{
			name:    "boot",
			newData: newBoot,
			oldData: oldBoot,
			ops: []opSpec{
				{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('N'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}},
				{
					typ:     chromeos_update_engine.InstallOperation_SOURCE_COPY,
					src:     []*chromeos_update_engine.Extent{ext(0, 1)},
					dst:     []*chromeos_update_engine.Extent{ext(2, 1)},
					srcHash: sum(oldBoot[0:blockSize]),
				},
			},
		},
	})
	p := openPayload(t, data)
	srcDir := writeSourceDir(t, map[string][]byte{"boot": oldBoot})
	outDir := t.TempDir()
	if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, SourceDir: srcDir}); err != nil {
		t.Fatal(err)
	}
	if got := readImage(t, outDir, "boot"); !bytes.Equal(got, newBoot) {
		t.Fatal("uncovered blocks must fall through to source image content")
	}
}

func buildVerityPartition(t *testing.T, oldData []byte) ([]byte, []byte) {
	t.Helper()
	salt := bytes.Repeat([]byte{0xAB}, 32)
	dataRegion := concat(block('1'), block('2'))
	level0 := make([]byte, 0, 2*32)
	for i := 0; i < 2; i++ {
		h := sha256.New()
		h.Write(salt)
		h.Write(dataRegion[i*blockSize : (i+1)*blockSize])
		level0 = append(level0, h.Sum(nil)...)
	}
	tree := append(level0, make([]byte, blockSize-len(level0))...)
	newData := concat(dataRegion, tree)
	payloadBytes := buildPayloadWith(t, 9, []partitionSpec{
		{
			name:    "vendor",
			newData: newData,
			oldData: oldData,
			ops: []opSpec{
				{
					typ:     chromeos_update_engine.InstallOperation_SOURCE_COPY,
					src:     []*chromeos_update_engine.Extent{ext(0, 1)},
					dst:     []*chromeos_update_engine.Extent{ext(0, 1)},
					srcHash: sum(oldData[0:blockSize]),
				},
				{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('2'), dst: []*chromeos_update_engine.Extent{ext(1, 1)}},
			},
		},
	}, func(update *chromeos_update_engine.PartitionUpdate) {
		update.HashTreeDataExtent = ext(0, 2)
		update.HashTreeExtent = ext(2, 1)
		update.HashTreeAlgorithm = proto.String("sha256")
		update.HashTreeSalt = salt
	})
	return payloadBytes, newData
}

func TestExtractDeltaWritesHashTree(t *testing.T) {
	oldData := concat(block('1'), block('x'), make([]byte, blockSize))
	payloadBytes, newData := buildVerityPartition(t, oldData)
	p := openPayload(t, payloadBytes)
	srcDir := writeSourceDir(t, map[string][]byte{"vendor": oldData})
	outDir := t.TempDir()
	if err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: outDir, SourceDir: srcDir}); err != nil {
		t.Fatal(err)
	}
	if got := readImage(t, outDir, "vendor"); !bytes.Equal(got, newData) {
		t.Fatal("computed dm-verity hash tree does not match expected tree")
	}
}

func TestExtractDeltaFecUnsupported(t *testing.T) {
	oldData := concat(block('o'), block('o'), block('o'))
	newData := concat(block('o'), block('n'), make([]byte, blockSize))
	data := buildPayloadWith(t, 9, []partitionSpec{
		{
			name:    "vendor",
			newData: newData,
			oldData: oldData,
			ops: []opSpec{
				{
					typ:     chromeos_update_engine.InstallOperation_SOURCE_COPY,
					src:     []*chromeos_update_engine.Extent{ext(0, 1)},
					dst:     []*chromeos_update_engine.Extent{ext(0, 1)},
					srcHash: sum(oldData[0:blockSize]),
				},
				{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('n'), dst: []*chromeos_update_engine.Extent{ext(1, 1)}},
			},
		},
	}, func(update *chromeos_update_engine.PartitionUpdate) {
		update.FecDataExtent = ext(0, 2)
		update.FecExtent = ext(2, 1)
		update.FecRoots = proto.Uint32(2)
	})
	p := openPayload(t, data)
	srcDir := writeSourceDir(t, map[string][]byte{"vendor": oldData})
	err := p.Extract(context.Background(), payload.ExtractOptions{OutputDir: t.TempDir(), SourceDir: srcDir})
	if err == nil || !strings.Contains(err.Error(), "FEC") {
		t.Fatalf("expected FEC unsupported error, got %v", err)
	}
}

func TestExtractCancellation(t *testing.T) {
	data := buildPayload(t, 0, []partitionSpec{
		{name: "boot", newData: block('A'), ops: []opSpec{{typ: chromeos_update_engine.InstallOperation_REPLACE, data: block('A'), dst: []*chromeos_update_engine.Extent{ext(0, 1)}}}},
	})
	p := openPayload(t, data)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Extract(ctx, payload.ExtractOptions{OutputDir: t.TempDir()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
