package payload

import (
	"io"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

// Logger defines interface for logging messages
type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
}

// ProgressReporter defines interface for reporting extraction progress
type ProgressReporter interface {
	StartExtraction(partitionName string, totalOperations int, size uint64) ProgressBar
	Finish()
}

// ProgressBar represents a single partition's progress
type ProgressBar interface {
	Increment()
	SetTotal(total int64, complete bool)
}

// ExtractOptions contains options for extraction
type ExtractOptions struct {
	OutputDirectory  string
	Concurrency      int
	SelectedParts    []string
	ProgressReporter ProgressReporter
	Logger           Logger
	QuietMode        bool
	MachineReadable  bool
}

// Config contains global configuration for payload operations
type Config struct {
	Concurrency     int
	QuietMode       bool
	MachineReadable bool
	Logger          Logger
	ProgressReporter ProgressReporter
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Concurrency:     4,
		QuietMode:       false,
		MachineReadable: false,
	}
}

// PartitionInfo contains information about a partition
type PartitionInfo struct {
	Name string
	Size uint64
}

// PayloadReader defines interface for reading payload files
type PayloadReader interface {
	Open() error
	Init() error
	GetPartitions() []PartitionInfo
	ExtractAll(targetDirectory string) error
	ExtractSelected(targetDirectory string, partitions []string) error
	ExtractWithOptions(options ExtractOptions) error
	SetConcurrency(concurrency int)
	GetConcurrency() int
	SetConfig(config *Config)
	GetConfig() *Config
	Close() error
}

// PartitionExtractor defines interface for extracting individual partitions
type PartitionExtractor interface {
	Extract(partition *chromeos_update_engine.PartitionUpdate, out io.Writer) error
}

// PayloadHeader represents the payload file header
type PayloadHeader struct {
	Version              uint64
	ManifestLen          uint64
	MetadataSignatureLen uint32
	Size                 uint64
}

// ExtractRequest represents a request to extract a partition
type ExtractRequest struct {
	Partition       *chromeos_update_engine.PartitionUpdate
	TargetDirectory string
}

const (
	PayloadHeaderMagic        = "CrAU"
	BrilloMajorPayloadVersion = 2
	BlockSize                 = 4096
)