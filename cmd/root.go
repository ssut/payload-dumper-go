package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"

	"github.com/ssut/payload-dumper-go/payload"
)

func Execute() int {
	var (
		list            bool
		partitions      string
		outputDirectory string
		oldDirectory    string
		concurrency     int
		quiet           bool
		machineReadable bool
		noVerify        bool
	)

	flag.IntVar(&concurrency, "c", runtime.NumCPU(), "Number of workers to extract concurrently (shorthand)")
	flag.IntVar(&concurrency, "concurrency", runtime.NumCPU(), "Number of workers to extract concurrently")
	flag.BoolVar(&list, "l", false, "Show list of partitions in payload.bin (shorthand)")
	flag.BoolVar(&list, "list", false, "Show list of partitions in payload.bin")
	flag.StringVar(&outputDirectory, "o", "", "Set output directory (shorthand)")
	flag.StringVar(&outputDirectory, "output", "", "Set output directory")
	flag.StringVar(&partitions, "p", "", "Dump only selected partitions (comma-separated) (shorthand)")
	flag.StringVar(&partitions, "partitions", "", "Dump only selected partitions (comma-separated)")
	flag.StringVar(&oldDirectory, "old", "", "Directory with source images (<name>.img) required to apply an incremental (delta) payload")
	flag.BoolVar(&quiet, "q", false, "Quiet mode - suppress non-essential output (shorthand)")
	flag.BoolVar(&quiet, "quiet", false, "Quiet mode - suppress non-essential output")
	flag.BoolVar(&machineReadable, "m", false, "Machine-readable output format (shorthand)")
	flag.BoolVar(&machineReadable, "machine-readable", false, "Machine-readable output format")
	flag.BoolVar(&noVerify, "no-verify", false, "Skip source/output sha256 verification")
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		return 2
	}
	filename := flag.Arg(0)

	pl, err := payload.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer pl.Close()
	pl.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	parts := pl.Partitions()
	needsSource := false
	for _, part := range parts {
		if part.NeedsSource {
			needsSource = true
			break
		}
	}

	verbose := !quiet && !machineReadable
	if verbose {
		fmt.Printf("payload.bin: %s\n", filename)
		fmt.Printf("Payload Version: %d\n", pl.Header().Version)
		fmt.Printf("Payload Manifest Length: %d\n", pl.Header().ManifestLen)
		fmt.Printf("Payload Manifest Signature Length: %d\n", pl.Header().MetadataSignatureLen)
		if needsSource {
			fmt.Printf("Delta payload: minor version %d (source images required, pass -old)\n", pl.MinorVersion())
		} else if pl.IsDelta() {
			fmt.Printf("Partial update payload: minor version %d (no source images required)\n", pl.MinorVersion())
		}
	}
	if list {
		printPartitionList(parts, machineReadable)
		return 0
	}
	if verbose {
		printPartitionList(parts, false)
	}

	targetDirectory := outputDirectory
	if targetDirectory == "" {
		now := time.Now()
		targetDirectory = fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d",
			now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}

	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	if verbose {
		fmt.Printf("Number of workers: %d\n", concurrency)
	}

	var selected []string
	if partitions != "" {
		selected = strings.Split(partitions, ",")
	}

	var onProgress payload.ProgressFunc
	var bars *barRenderer
	if machineReadable && !quiet {
		onProgress = newMachineRenderer().handle
	} else if !quiet {
		bars = newBarRenderer(parts)
		onProgress = bars.handle
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = pl.Extract(ctx, payload.ExtractOptions{
		OutputDir:   targetDirectory,
		Partitions:  selected,
		Concurrency: concurrency,
		SourceDir:   oldDirectory,
		SkipVerify:  noVerify,
		OnProgress:  onProgress,
	})
	if bars != nil {
		bars.wait()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		var missingSource *payload.MissingSourceError
		if errors.As(err, &missingSource) {
			fmt.Fprintf(os.Stderr, "Hint: extract the previous (base) full OTA first, then pass its output directory via -old, e.g.:\n")
			fmt.Fprintf(os.Stderr, "  payload-dumper-go -o base_images base_ota.zip\n")
			fmt.Fprintf(os.Stderr, "  payload-dumper-go -old base_images -o new_images %s\n", filename)
		}
		return 1
	}
	return 0
}

func printPartitionList(parts []payload.Partition, machineReadable bool) {
	if machineReadable {
		for _, part := range parts {
			fmt.Printf("%s:%d\n", part.Name, part.Size/1024)
		}
		return
	}
	fmt.Println("Found partitions:")
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		entry := fmt.Sprintf("%s (%s)", part.Name, humanize.Bytes(part.Size))
		if part.NeedsSource {
			entry = fmt.Sprintf("%s (%s, delta)", part.Name, humanize.Bytes(part.Size))
		}
		entries = append(entries, entry)
	}
	fmt.Println(strings.Join(entries, ", "))
}

type barRenderer struct {
	mu       sync.Mutex
	progress *mpb.Progress
	bars     map[string]*mpb.Bar
	last     map[string]int
	sizes    map[string]uint64
}

func newBarRenderer(parts []payload.Partition) *barRenderer {
	sizes := make(map[string]uint64, len(parts))
	for _, part := range parts {
		sizes[part.Name] = part.Size
	}
	return &barRenderer{
		progress: mpb.New(mpb.WithOutput(os.Stderr)),
		bars:     map[string]*mpb.Bar{},
		last:     map[string]int{},
		sizes:    sizes,
	}
}

func (r *barRenderer) handle(ev payload.ProgressEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bar, ok := r.bars[ev.Partition]
	if !ok {
		name := fmt.Sprintf("%s (%s)", ev.Partition, humanize.Bytes(r.sizes[ev.Partition]))
		bar = r.progress.AddBar(
			int64(ev.TotalOps),
			mpb.PrependDecorators(decor.Name(name, decor.WCSyncSpaceR)),
			mpb.AppendDecorators(decor.Percentage()),
		)
		r.bars[ev.Partition] = bar
	}
	if delta := ev.CompletedOps - r.last[ev.Partition]; delta > 0 {
		bar.IncrBy(delta)
		r.last[ev.Partition] = ev.CompletedOps
	}
	if ev.Err != nil {
		bar.Abort(false)
		return
	}
	if ev.Done {
		bar.SetTotal(int64(ev.TotalOps), true)
	}
}

func (r *barRenderer) wait() {
	r.progress.Wait()
}

type machineRenderer struct {
	mu   sync.Mutex
	last map[string]int
}

func newMachineRenderer() *machineRenderer {
	return &machineRenderer{last: map[string]int{}}
}

func (r *machineRenderer) handle(ev payload.ProgressEvent) {
	if ev.Err != nil {
		return
	}
	percent := 100
	if ev.TotalOps > 0 {
		percent = ev.CompletedOps * 100 / ev.TotalOps
	}
	if ev.Done {
		percent = 100
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if last, ok := r.last[ev.Partition]; ok && percent <= last {
		return
	}
	r.last[ev.Partition] = percent
	fmt.Printf("%s:%d\n", ev.Partition, percent)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] [inputfile]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nInput can be a payload.bin or an OTA zip containing payload.bin.\n")
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  %s ota.zip                                  # Extract all partitions from a full OTA\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -l payload.bin                           # List partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -p boot,system payload.bin               # Extract selected partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -old base_images incremental_ota.zip     # Apply an incremental OTA on top of base images\n", os.Args[0])
}
