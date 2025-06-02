// Package payload provides functionality for reading and extracting Android OTA payload files.
//
// This package supports extracting partitions from Android OTA update files (payload.bin)
// with support for various compression formats including XZ, BZIP2, ZSTD, and uncompressed data.
//
// Basic usage:
//
//	reader := payload.New("payload.bin")
//	if err := reader.Open(); err != nil {
//		log.Fatal(err)
//	}
//	defer reader.Close()
//
//	if err := reader.Init(); err != nil {
//		log.Fatal(err)
//	}
//
//	// Get partition information
//	partitions := reader.GetPartitions()
//	
//	// Extract all partitions
//	if err := reader.ExtractAll("output_directory"); err != nil {
//		log.Fatal(err)
//	}
//
// The package supports custom logging and progress reporting through interfaces:
//
//	// Custom logger
//	reader.SetLogger(myLogger)
//	
//	// Custom progress reporter
//	reader.SetProgressReporter(myProgressReporter)
//
// For more control over extraction, you can specify which partitions to extract:
//
//	selectedPartitions := []string{"boot", "system", "vendor"}
//	err := reader.ExtractSelected("output_directory", selectedPartitions)
package payload