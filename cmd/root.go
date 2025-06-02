package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"

	"github.com/ssut/payload-dumper-go/internal/archive"
	"github.com/ssut/payload-dumper-go/payload"
)

// StandardLogger implements payload.Logger using standard log package
type StandardLogger struct {
	logger *log.Logger
	quiet  bool
}

// NewStandardLogger creates a new standard logger
func NewStandardLogger(quiet bool) *StandardLogger {
	return &StandardLogger{
		logger: log.New(os.Stdout, "", 0),
		quiet:  quiet,
	}
}

// Printf logs a formatted message
func (sl *StandardLogger) Printf(format string, args ...interface{}) {
	if !sl.quiet {
		sl.logger.Printf(format, args...)
	}
}

// Println logs a message
func (sl *StandardLogger) Println(args ...interface{}) {
	if !sl.quiet {
		sl.logger.Println(args...)
	}
}

// MachineReadableProgressReporter implements machine-readable output
type MachineReadableProgressReporter struct {
	quiet bool
}

// NewMachineReadableProgressReporter creates a machine-readable progress reporter
func NewMachineReadableProgressReporter(quiet bool) *MachineReadableProgressReporter {
	return &MachineReadableProgressReporter{quiet: quiet}
}

// StartExtraction starts progress tracking for a partition in machine-readable format
func (mr *MachineReadableProgressReporter) StartExtraction(partitionName string, totalOperations int, size uint64) payload.ProgressBar {
	return &MachineReadableProgressBar{
		name:  partitionName,
		total: totalOperations,
		quiet: mr.quiet,
	}
}

// Finish completes all progress tracking
func (mr *MachineReadableProgressReporter) Finish() {
	// No output needed for machine-readable mode
}

// MachineReadableProgressBar implements machine-readable progress output
type MachineReadableProgressBar struct {
	name      string
	total     int
	completed int
	quiet     bool
}

// Increment increments the progress bar and outputs machine-readable progress
func (mb *MachineReadableProgressBar) Increment() {
	mb.completed++
	if !mb.quiet {
		percentage := (mb.completed * 100) / mb.total
		fmt.Printf("%s:%d\n", mb.name, percentage)
	}
}

// SetTotal sets the total and completion status
func (mb *MachineReadableProgressBar) SetTotal(total int64, complete bool) {
	if complete && !mb.quiet {
		fmt.Printf("%s:100\n", mb.name)
	}
}

// ProgressReporter implements payload.ProgressReporter using mpb
type ProgressReporter struct {
	progress *mpb.Progress
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{
		progress: mpb.New(),
	}
}

// StartExtraction starts progress tracking for a partition
func (pr *ProgressReporter) StartExtraction(partitionName string, totalOperations int, size uint64) payload.ProgressBar {
	var barName string
	if size > 0 {
		barName = fmt.Sprintf("%s (%s)", partitionName, humanize.Bytes(size))
	} else {
		barName = partitionName
	}
	bar := pr.progress.AddBar(
		int64(totalOperations),
		mpb.PrependDecorators(
			decor.Name(barName, decor.WCSyncSpaceR),
		),
		mpb.AppendDecorators(
			decor.Percentage(),
		),
	)
	return &ProgressBarWrapper{bar: bar}
}

// Finish completes all progress tracking
func (pr *ProgressReporter) Finish() {
	pr.progress.Wait()
}

// ProgressBarWrapper wraps mpb.Bar to implement payload.ProgressBar
type ProgressBarWrapper struct {
	bar *mpb.Bar
}

// Increment increments the progress bar
func (pb *ProgressBarWrapper) Increment() {
	pb.bar.Increment()
}

// SetTotal sets the total and completion status
func (pb *ProgressBarWrapper) SetTotal(total int64, complete bool) {
	pb.bar.SetTotal(total, complete)
}

// Execute runs the main CLI application
func Execute() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		list            bool
		partitions      string
		outputDirectory string
		concurrency     int
		quiet           bool
		machineReadable bool
	)

	flag.IntVar(&concurrency, "c", runtime.NumCPU(), "Number of multiple workers to extract (default: all CPU cores)")
	flag.IntVar(&concurrency, "concurrency", runtime.NumCPU(), "Number of multiple workers to extract (default: all CPU cores)")
	flag.BoolVar(&list, "l", false, "Show list of partitions in payload.bin (shorthand)")
	flag.BoolVar(&list, "list", false, "Show list of partitions in payload.bin")
	flag.StringVar(&outputDirectory, "o", "", "Set output directory (shorthand)")
	flag.StringVar(&outputDirectory, "output", "", "Set output directory")
	flag.StringVar(&partitions, "p", "", "Dump only selected partitions (comma-separated) (shorthand)")
	flag.StringVar(&partitions, "partitions", "", "Dump only selected partitions (comma-separated)")
	flag.BoolVar(&quiet, "q", false, "Quiet mode - suppress non-essential output (shorthand)")
	flag.BoolVar(&quiet, "quiet", false, "Quiet mode - suppress non-essential output")
	flag.BoolVar(&machineReadable, "m", false, "Machine-readable output format (shorthand)")
	flag.BoolVar(&machineReadable, "machine-readable", false, "Machine-readable output format")
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
	}
	filename := flag.Arg(0)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File does not exist: %s\n", filename)
		os.Exit(1)
	}

	payloadBin := filename
	var tempFile string

	if strings.HasSuffix(filename, ".zip") {
		if !quiet {
			fmt.Println("Please wait while extracting payload.bin from the archive.")
		}
		var err error
		payloadBin, err = archive.ExtractPayloadFromZip(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		tempFile = payloadBin
		defer archive.CleanupTempFile(tempFile)
	}
	if !quiet {
		fmt.Printf("payload.bin: %s\n", payloadBin)
	}

	payloadReader := payload.New(payloadBin)

	// Set up configuration
	config := payload.DefaultConfig()
	config.Concurrency = concurrency
	config.QuietMode = quiet
	config.MachineReadable = machineReadable

	// Set up logging
	logger := NewStandardLogger(quiet)
	config.Logger = logger

	// Set up progress reporting based on mode
	if machineReadable {
		config.ProgressReporter = NewMachineReadableProgressReporter(quiet)
	} else if !quiet {
		config.ProgressReporter = NewProgressReporter()
	}

	payloadReader.SetConfig(config)

	if err := payloadReader.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening payload: %v\n", err)
		os.Exit(1)
	}
	defer payloadReader.Close()

	if err := payloadReader.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing payload: %v\n", err)
		os.Exit(1)
	}

	// Print payload information
	partitionInfos := payloadReader.GetPartitions()

	if list {
		if machineReadable {
			// Machine-readable format: partition:size-in-KB
			for _, partitionInfo := range partitionInfos {
				sizeKB := partitionInfo.Size / 1024
				fmt.Printf("%s:%d\n", partitionInfo.Name, sizeKB)
			}
		} else if !quiet {
			fmt.Println("Found partitions:")
			for i, partitionInfo := range partitionInfos {
				fmt.Printf("%s (%s)", partitionInfo.Name, humanize.Bytes(partitionInfo.Size))
				if i < len(partitionInfos)-1 {
					fmt.Printf(", ")
				} else {
					fmt.Printf("\n")
				}
			}
		}
		return
	}

	if !quiet && !machineReadable {
		fmt.Println("Found partitions:")
		for i, partitionInfo := range partitionInfos {
			fmt.Printf("%s (%s)", partitionInfo.Name, humanize.Bytes(partitionInfo.Size))
			if i < len(partitionInfos)-1 {
				fmt.Printf(", ")
			} else {
				fmt.Printf("\n")
			}
		}
	}

	now := time.Now()
	targetDirectory := outputDirectory
	if targetDirectory == "" {
		targetDirectory = fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d",
			now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}

	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		if err := os.Mkdir(targetDirectory, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create target directory: %v\n", err)
			os.Exit(1)
		}
	}

	if !quiet {
		fmt.Printf("Number of workers: %d\n", payloadReader.GetConcurrency())
	}

	var err error
	if partitions != "" {
		err = payloadReader.ExtractSelected(targetDirectory, strings.Split(partitions, ","))
	} else {
		err = payloadReader.ExtractAll(targetDirectory)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Extraction failed: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] [inputfile]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  %s payload.bin                           # Extract all partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -l payload.bin                        # List partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -p boot,system payload.bin            # Extract specific partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -q -c 8 payload.bin                   # Quiet mode with 8 workers\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -m -l payload.bin                     # Machine-readable partition list\n", os.Args[0])
	os.Exit(2)
}
