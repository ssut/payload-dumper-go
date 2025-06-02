# Payload Dumper Go - Usage Guide

This document explains how to use payload-dumper-go both as a CLI tool and as a library.

## CLI Usage

The CLI interface maintains full backward compatibility with the original implementation.

### Basic Usage

```bash
# Extract all partitions from payload.bin
./payload-dumper-go payload.bin

# Extract from a ZIP file containing payload.bin
./payload-dumper-go update.zip

# List partitions without extracting
./payload-dumper-go -l payload.bin

# Extract specific partitions
./payload-dumper-go -p "boot,system,vendor" payload.bin

# Specify output directory
./payload-dumper-go -o /path/to/output payload.bin

# Use multiple workers for faster extraction
./payload-dumper-go -c 8 payload.bin
```

### CLI Options

- `-c, -concurrency`: Number of worker threads (default: 4)
- `-l, -list`: Show list of partitions without extracting
- `-o, -output`: Set output directory
- `-p, -partitions`: Extract only specified partitions (comma-separated)

## Library Usage

The refactored codebase now provides a clean API for using payload-dumper-go as a library in your Go applications.

### Basic Library Usage

```go
package main

import (
    "log"
    "github.com/ssut/payload-dumper-go/payload"
)

func main() {
    // Create a new payload reader
    reader := payload.New("payload.bin")
    
    // Open the payload file
    if err := reader.Open(); err != nil {
        log.Fatal(err)
    }
    defer reader.Close()
    
    // Initialize (reads header and manifest)
    if err := reader.Init(); err != nil {
        log.Fatal(err)
    }
    
    // Extract all partitions
    if err := reader.ExtractAll("output_directory"); err != nil {
        log.Fatal(err)
    }
}
```

### Advanced Library Usage

```go
package main

import (
    "fmt"
    "log"
    "github.com/ssut/payload-dumper-go/payload"
)

// Custom logger implementation
type MyLogger struct{}

func (l *MyLogger) Printf(format string, args ...interface{}) {
    fmt.Printf("[PAYLOAD] "+format+"\n", args...)
}

func (l *MyLogger) Println(args ...interface{}) {
    fmt.Print("[PAYLOAD] ")
    fmt.Println(args...)
}

// Custom progress reporter
type MyProgressReporter struct{}

func (r *MyProgressReporter) StartExtraction(partitionName string, totalOperations int, size uint64) payload.ProgressBar {
    fmt.Printf("Starting %s extraction (%d operations)\n", partitionName, totalOperations)
    return &MyProgressBar{name: partitionName, total: totalOperations}
}

func (r *MyProgressReporter) Finish() {
    fmt.Println("All extractions completed!")
}

type MyProgressBar struct {
    name    string
    total   int
    current int
}

func (b *MyProgressBar) Increment() {
    b.current++
    if b.current%10 == 0 {
        fmt.Printf("  %s: %d/%d\n", b.name, b.current, b.total)
    }
}

func (b *MyProgressBar) SetTotal(total int64, complete bool) {
    if complete {
        fmt.Printf("  %s: completed\n", b.name)
    }
}

func main() {
    reader := payload.New("payload.bin")
    
    // Set custom logger
    reader.SetLogger(&MyLogger{})
    
    // Set custom progress reporter
    reader.SetProgressReporter(&MyProgressReporter{})
    
    if err := reader.Open(); err != nil {
        log.Fatal(err)
    }
    defer reader.Close()
    
    if err := reader.Init(); err != nil {
        log.Fatal(err)
    }
    
    // Get partition information
    partitions := reader.GetPartitions()
    fmt.Printf("Found %d partitions:\n", len(partitions))
    for _, p := range partitions {
        fmt.Printf("  - %s (%d bytes)\n", p.Name, p.Size)
    }
    
    // Configure extraction settings
    reader.SetConcurrency(8)
    
    // Extract specific partitions
    selectedPartitions := []string{"boot", "system", "vendor"}
    if err := reader.ExtractSelected("output", selectedPartitions); err != nil {
        log.Fatal(err)
    }
}
```

## Archive Support

The library automatically handles ZIP files containing payload.bin:

```go
// For ZIP files, use the internal/archive package
import "github.com/ssut/payload-dumper-go/internal/archive"

payloadPath, err := archive.ExtractPayloadFromZip("update.zip")
if err != nil {
    log.Fatal(err)
}
defer archive.CleanupTempFile(payloadPath)

reader := payload.New(payloadPath)
// ... continue as normal
```

## Interfaces

### Logger Interface

Implement this interface to customize logging:

```go
type Logger interface {
    Printf(format string, args ...interface{})
    Println(args ...interface{})
}
```

### ProgressReporter Interface

Implement this interface to customize progress reporting:

```go
type ProgressReporter interface {
    StartExtraction(partitionName string, totalOperations int, size uint64) ProgressBar
    Finish()
}

type ProgressBar interface {
    Increment()
    SetTotal(total int64, complete bool)
}
```

## Building

To build the project, you need the XZ development libraries installed:

### macOS (with Homebrew)
```bash
CGO_CFLAGS="-I/opt/homebrew/include" CGO_LDFLAGS="-L/opt/homebrew/lib" go build
```

### Linux (Ubuntu/Debian)
```bash
sudo apt-get install liblzma-dev
go build
```

### Linux (CentOS/RHEL)
```bash
sudo yum install xz-devel
go build
```

## Supported Compression Formats

- Uncompressed (REPLACE)
- XZ (REPLACE_XZ)
- BZIP2 (REPLACE_BZ)
- ZSTD (ZSTD)
- Zero-filled blocks (ZERO)

## Error Handling

The library provides detailed error messages for various failure scenarios:

- Invalid payload files
- Unsupported compression formats
- Checksum verification failures
- File I/O errors
- Insufficient disk space

All extraction operations include hash verification to ensure data integrity.