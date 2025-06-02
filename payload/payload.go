package payload

import (
	"errors"
	"os"
	"sort"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
)

// Payload represents a payload file processor
type Payload struct {
	filename string
	file     *os.File

	header               *Header
	deltaArchiveManifest *chromeos_update_engine.DeltaArchiveManifest
	signatures           *chromeos_update_engine.Signatures

	config       *Config
	metadataSize int64
	dataOffset   int64
	initialized  bool

	requests chan *ExtractRequest
	workerWG sync.WaitGroup
}

// New creates a new Payload instance
func New(filename string) *Payload {
	return &Payload{
		filename: filename,
		config:   DefaultConfig(),
	}
}

// Open opens the payload file
func (p *Payload) Open() error {
	file, err := os.Open(p.filename)
	if err != nil {
		return err
	}
	p.file = file
	return nil
}

// Close closes the payload file
func (p *Payload) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}

// Init initializes the payload by reading header and manifest
func (p *Payload) Init() error {
	// Read header
	header, err := ReadHeader(p.file)
	if err != nil {
		return err
	}
	p.header = header

	// Log header info
	if p.config.Logger != nil && !p.config.QuietMode {
		p.config.Logger.Printf("Payload Version: %d", p.header.Version)
		p.config.Logger.Printf("Payload Manifest Length: %d", p.header.ManifestLen)
		p.config.Logger.Printf("Payload Manifest Signature Length: %d", p.header.MetadataSignatureLen)
	}

	// Read manifest
	deltaArchiveManifest, err := p.readManifest()
	if err != nil {
		return err
	}
	p.deltaArchiveManifest = deltaArchiveManifest

	// Read signatures
	signatures, err := p.readMetadataSignature()
	if err != nil {
		return err
	}
	p.signatures = signatures

	// Update offsets
	p.metadataSize = int64(p.header.Size + p.header.ManifestLen)
	p.dataOffset = p.metadataSize + int64(p.header.MetadataSignatureLen)

	p.initialized = true
	return nil
}

// GetPartitions returns information about all partitions
func (p *Payload) GetPartitions() []PartitionInfo {
	if !p.initialized {
		return nil
	}

	partitions := make([]PartitionInfo, len(p.deltaArchiveManifest.Partitions))
	for i, partition := range p.deltaArchiveManifest.Partitions {
		partitions[i] = PartitionInfo{
			Name: partition.GetPartitionName(),
			Size: partition.GetNewPartitionInfo().GetSize(),
		}
	}
	return partitions
}

// SetConcurrency sets the number of worker goroutines
func (p *Payload) SetConcurrency(concurrency int) {
	p.config.Concurrency = concurrency
}

// GetConcurrency returns the number of worker goroutines
func (p *Payload) GetConcurrency() int {
	return p.config.Concurrency
}

// SetConfig sets the configuration
func (p *Payload) SetConfig(config *Config) {
	p.config = config
}

// GetConfig returns the current configuration
func (p *Payload) GetConfig() *Config {
	return p.config
}

// ExtractAll extracts all partitions to the target directory
func (p *Payload) ExtractAll(targetDirectory string) error {
	return p.ExtractSelected(targetDirectory, nil)
}

// ExtractSelected extracts selected partitions to the target directory
func (p *Payload) ExtractSelected(targetDirectory string, partitions []string) error {
	options := ExtractOptions{
		OutputDirectory: targetDirectory,
		SelectedParts:   partitions,
		Concurrency:     p.config.Concurrency,
		QuietMode:       p.config.QuietMode,
		MachineReadable: p.config.MachineReadable,
		Logger:          p.config.Logger,
		ProgressReporter: p.config.ProgressReporter,
	}
	return p.ExtractWithOptions(options)
}

// ExtractWithOptions extracts partitions using the provided options
func (p *Payload) ExtractWithOptions(options ExtractOptions) error {
	if !p.initialized {
		return errors.New("payload has not been initialized")
	}

	// Use options or fallback to config
	concurrency := options.Concurrency
	if concurrency == 0 {
		concurrency = p.config.Concurrency
	}

	reporter := options.ProgressReporter
	if reporter == nil {
		reporter = p.config.ProgressReporter
	}

	if reporter != nil {
		defer reporter.Finish()
	}

	p.requests = make(chan *ExtractRequest, 100)
	p.spawnExtractWorkers(concurrency)

	partitions := options.SelectedParts
	if partitions != nil {
		sort.Strings(partitions)
	}

	for _, partition := range p.deltaArchiveManifest.Partitions {
		if len(partitions) > 0 {
			idx := sort.SearchStrings(partitions, partition.GetPartitionName())
			if idx == len(partitions) || partitions[idx] != partition.GetPartitionName() {
				continue
			}
		}

		p.workerWG.Add(1)
		p.requests <- &ExtractRequest{
			Partition:       partition,
			TargetDirectory: options.OutputDirectory,
		}
	}

	p.workerWG.Wait()
	close(p.requests)

	return nil
}

// SetProgressReporter sets the progress reporter
func (p *Payload) SetProgressReporter(reporter ProgressReporter) {
	p.config.ProgressReporter = reporter
}

// SetLogger sets the logger for the payload
func (p *Payload) SetLogger(logger Logger) {
	p.config.Logger = logger
}

func (p *Payload) readManifest() (*chromeos_update_engine.DeltaArchiveManifest, error) {
	buf := make([]byte, p.header.ManifestLen)
	if _, err := p.file.Read(buf); err != nil {
		return nil, err
	}
	
	deltaArchiveManifest := &chromeos_update_engine.DeltaArchiveManifest{}
	if err := proto.Unmarshal(buf, deltaArchiveManifest); err != nil {
		return nil, err
	}

	return deltaArchiveManifest, nil
}

func (p *Payload) readMetadataSignature() (*chromeos_update_engine.Signatures, error) {
	if _, err := p.file.Seek(int64(p.header.Size+p.header.ManifestLen), 0); err != nil {
		return nil, err
	}

	buf := make([]byte, p.header.MetadataSignatureLen)
	if _, err := p.file.Read(buf); err != nil {
		return nil, err
	}
	
	signatures := &chromeos_update_engine.Signatures{}
	if err := proto.Unmarshal(buf, signatures); err != nil {
		return nil, err
	}

	return signatures, nil
}

func (p *Payload) worker() {
	extractor := NewExtractor(p.file, p.dataOffset)

	for req := range p.requests {
		partition := req.Partition
		targetDirectory := req.TargetDirectory

		name := partition.GetPartitionName() + ".img"
		filepath := targetDirectory + "/" + name
		
		file, err := os.OpenFile(filepath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o755)
		if err != nil {
			if p.config.Logger != nil && !p.config.QuietMode {
				p.config.Logger.Printf("Error creating file %s: %v", filepath, err)
			}
			p.workerWG.Done()
			continue
		}

		var progressBar ProgressBar
		if p.config.ProgressReporter != nil {
			progressBar = p.config.ProgressReporter.StartExtraction(partition.GetPartitionName(), len(partition.Operations), partition.GetNewPartitionInfo().GetSize())
		}

		if err := extractor.Extract(partition, file, progressBar); err != nil {
			if p.config.Logger != nil && !p.config.QuietMode {
				p.config.Logger.Printf("Error extracting partition %s: %v", partition.GetPartitionName(), err)
			}
		}

		if progressBar != nil {
			progressBar.SetTotal(0, true)
		}

		file.Close()
		p.workerWG.Done()
	}
}

func (p *Payload) spawnExtractWorkers(n int) {
	for i := 0; i < n; i++ {
		go p.worker()
	}
}