package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ssut/payload-dumper-go/payload"
)

// ExampleLogger demonstrates a custom logger implementation
type ExampleLogger struct {
	prefix string
}

func NewExampleLogger(prefix string) *ExampleLogger {
	return &ExampleLogger{prefix: prefix}
}

func (el *ExampleLogger) Printf(format string, args ...interface{}) {
	fmt.Printf("[%s] "+format+"\n", append([]interface{}{el.prefix}, args...)...)
}

func (el *ExampleLogger) Println(args ...interface{}) {
	fmt.Printf("[%s] ", el.prefix)
	fmt.Println(args...)
}

// ExampleProgressReporter demonstrates a simple progress reporter
type ExampleProgressReporter struct {
	machineReadable bool
}

func NewExampleProgressReporter(machineReadable bool) *ExampleProgressReporter {
	return &ExampleProgressReporter{machineReadable: machineReadable}
}

func (epr *ExampleProgressReporter) StartExtraction(partitionName string, totalOperations int, size uint64) payload.ProgressBar {
	if !epr.machineReadable {
		fmt.Printf("Starting extraction of %s (%d operations, %d bytes)\n", partitionName, totalOperations, size)
	}
	return &ExampleProgressBar{
		name:            partitionName,
		total:           totalOperations,
		machineReadable: epr.machineReadable,
	}
}

func (epr *ExampleProgressReporter) Finish() {
	if !epr.machineReadable {
		fmt.Println("All extractions completed!")
	}
}

type ExampleProgressBar struct {
	name            string
	total           int
	completed       int
	machineReadable bool
}

func (epb *ExampleProgressBar) Increment() {
	epb.completed++
	if epb.machineReadable {
		percentage := (epb.completed * 100) / epb.total
		fmt.Printf("%s:%d\n", epb.name, percentage)
	} else if epb.completed%10 == 0 || epb.completed == epb.total {
		fmt.Printf("  %s: %d/%d operations completed\n", epb.name, epb.completed, epb.total)
	}
}

func (epb *ExampleProgressBar) SetTotal(total int64, complete bool) {
	if complete {
		if epb.machineReadable {
			fmt.Printf("%s:100\n", epb.name)
		} else {
			fmt.Printf("  %s: extraction completed\n", epb.name)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run library_usage.go <payload.bin> [mode]")
		fmt.Println("Modes: basic, config, options, machine")
		fmt.Println("This example demonstrates how to use payload-dumper-go as a library")
		return
	}

	payloadFile := os.Args[1]
	mode := "basic"
	if len(os.Args) > 2 {
		mode = os.Args[2]
	}
	
	if _, err := os.Stat(payloadFile); os.IsNotExist(err) {
		log.Fatalf("File does not exist: %s", payloadFile)
	}

	fmt.Printf("=== Payload Dumper Go Library Usage Example (%s mode) ===\n", mode)

	switch mode {
	case "basic":
		basicUsage(payloadFile)
	case "config":
		configUsage(payloadFile)
	case "options":
		optionsUsage(payloadFile)
	case "machine":
		machineUsage(payloadFile)
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}

func basicUsage(payloadFile string) {
	fmt.Println("--- Basic Usage (Backward Compatible) ---")
	
	// Create a new payload reader
	reader := payload.New(payloadFile)

	// Set up custom logger (old way)
	logger := NewExampleLogger("BASIC")
	reader.SetLogger(logger)

	// Set up custom progress reporter (old way)
	progressReporter := NewExampleProgressReporter(false)
	reader.SetProgressReporter(progressReporter)

	// Open and initialize the payload
	if err := reader.Open(); err != nil {
		log.Fatalf("Failed to open payload: %v", err)
	}
	defer reader.Close()

	if err := reader.Init(); err != nil {
		log.Fatalf("Failed to initialize payload: %v", err)
	}

	// Get partition information
	partitions := reader.GetPartitions()
	fmt.Printf("Found %d partitions:\n", len(partitions))
	for _, partition := range partitions {
		fmt.Printf("  - %s (%d bytes)\n", partition.Name, partition.Size)
	}

	// Configure extraction (old way)
	reader.SetConcurrency(2)
	fmt.Printf("Using %d workers for extraction\n", reader.GetConcurrency())

	// Create output directory
	outputDir := "example_output_basic"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Extract first partition as an example
	if len(partitions) > 0 {
		selectedPartitions := []string{partitions[0].Name}
		fmt.Printf("Extracting partitions: %v\n", selectedPartitions)
		if err := reader.ExtractSelected(outputDir, selectedPartitions); err != nil {
			log.Fatalf("Extraction failed: %v", err)
		}
	}

	fmt.Println("Basic usage example completed!")
}

func configUsage(payloadFile string) {
	fmt.Println("--- Configuration Usage (Recommended) ---")
	
	reader := payload.New(payloadFile)

	// Create and configure using Config object
	config := payload.DefaultConfig()
	config.Concurrency = 4
	config.QuietMode = false
	config.MachineReadable = false
	config.Logger = NewExampleLogger("CONFIG")
	config.ProgressReporter = NewExampleProgressReporter(false)
	
	reader.SetConfig(config)

	if err := reader.Open(); err != nil {
		log.Fatalf("Failed to open payload: %v", err)
	}
	defer reader.Close()

	if err := reader.Init(); err != nil {
		log.Fatalf("Failed to initialize payload: %v", err)
	}

	partitions := reader.GetPartitions()
	fmt.Printf("Found %d partitions:\n", len(partitions))

	outputDir := "example_output_config"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	if len(partitions) > 0 {
		if err := reader.ExtractSelected(outputDir, []string{partitions[0].Name}); err != nil {
			log.Fatalf("Extraction failed: %v", err)
		}
	}

	fmt.Println("Configuration usage example completed!")
}

func optionsUsage(payloadFile string) {
	fmt.Println("--- ExtractWithOptions Usage (Maximum Control) ---")
	
	reader := payload.New(payloadFile)

	if err := reader.Open(); err != nil {
		log.Fatalf("Failed to open payload: %v", err)
	}
	defer reader.Close()

	if err := reader.Init(); err != nil {
		log.Fatalf("Failed to initialize payload: %v", err)
	}

	partitions := reader.GetPartitions()
	fmt.Printf("Found %d partitions:\n", len(partitions))

	outputDir := "example_output_options"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Use ExtractWithOptions for maximum control
	if len(partitions) > 0 {
		options := payload.ExtractOptions{
			OutputDirectory:  outputDir,
			Concurrency:      8,
			SelectedParts:    []string{partitions[0].Name},
			QuietMode:        false,
			MachineReadable:  false,
			Logger:           NewExampleLogger("OPTIONS"),
			ProgressReporter: NewExampleProgressReporter(false),
		}
		
		fmt.Printf("Extracting with options: concurrency=%d, partitions=%v\n", 
			options.Concurrency, options.SelectedParts)
		
		if err := reader.ExtractWithOptions(options); err != nil {
			log.Fatalf("Extraction failed: %v", err)
		}
	}

	fmt.Println("ExtractWithOptions usage example completed!")
}

func machineUsage(payloadFile string) {
	fmt.Println("--- Machine-Readable Usage ---")
	
	reader := payload.New(payloadFile)

	// Configure for machine-readable output
	config := payload.DefaultConfig()
	config.QuietMode = true
	config.MachineReadable = true
	config.ProgressReporter = NewExampleProgressReporter(true)
	
	reader.SetConfig(config)

	if err := reader.Open(); err != nil {
		log.Fatalf("Failed to open payload: %v", err)
	}
	defer reader.Close()

	if err := reader.Init(); err != nil {
		log.Fatalf("Failed to initialize payload: %v", err)
	}

	partitions := reader.GetPartitions()
	
	// Machine-readable partition list (format: partition:size-in-KB)
	fmt.Println("# Machine-readable partition list:")
	for _, partition := range partitions {
		sizeKB := partition.Size / 1024
		fmt.Printf("%s:%d\n", partition.Name, sizeKB)
	}

	outputDir := "example_output_machine"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Machine-readable extraction progress
	if len(partitions) > 0 {
		fmt.Println("# Machine-readable extraction progress:")
		if err := reader.ExtractSelected(outputDir, []string{partitions[0].Name}); err != nil {
			log.Fatalf("Extraction failed: %v", err)
		}
	}

	fmt.Println("# Machine-readable usage example completed!")
}