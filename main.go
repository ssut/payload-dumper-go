package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

func findPayloadBinOffset(filename string) (string, int64, error) {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		return "", 0, fmt.Errorf("not a valid zip archive: %s", filename)
	}
	defer zipReader.Close()

	for _, file := range zipReader.Reader.File {
		if file.Name == "payload.bin" && file.UncompressedSize64 > 0 {
			if file.Method != zip.Store {
				return "", 0, fmt.Errorf("payload.bin is compressed (method %d), expected STORE (0)", file.Method)
			}

			offset, err := file.DataOffset()
			if err != nil {
				return "", 0, fmt.Errorf("failed to get data offset: %w", err)
			}

			return filename, offset, nil
		}
	}

	return "", 0, fmt.Errorf("payload.bin not found in ZIP")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		list            bool
		partitions      string
		outputDirectory string
		concurrency     int
	)

	flag.IntVar(&concurrency, "c", 4, "Number of multiple workers to extract (shorthand)")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of multiple workers to extract")
	flag.BoolVar(&list, "l", false, "Show list of partitions in payload.bin (shorthand)")
	flag.BoolVar(&list, "list", false, "Show list of partitions in payload.bin")
	flag.StringVar(&outputDirectory, "o", "", "Set output directory (shorthand)")
	flag.StringVar(&outputDirectory, "output", "", "Set output directory")
	flag.StringVar(&partitions, "p", "", "Dump only selected partitions (comma-separated) (shorthand)")
	flag.StringVar(&partitions, "partitions", "", "Dump only selected partitions (comma-separated)")
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
	}
	filename := flag.Arg(0)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Fatalf("File does not exist: %s\n", filename)
	}

	payloadFile := filename
	payloadOffset := int64(0)
	
	if strings.HasSuffix(filename, ".zip") {
		fmt.Println("Locating payload.bin in ZIP archive...")
		zipFile, offset, err := findPayloadBinOffset(filename)
		if err != nil {
			log.Fatalf("Failed to locate payload.bin: %v\n", err)
		}
		payloadFile = zipFile
		payloadOffset = offset
		fmt.Printf("Found payload.bin at offset: %d\n", offset)
	}
	
	fmt.Printf("payload file: %s (offset: %d)\n", payloadFile, payloadOffset)

	payload := NewPayload(payloadFile, payloadOffset)
	if err := payload.Open(); err != nil {
		log.Fatal(err)
	}
	payload.Init()

	if list {
		return
	}

	now := time.Now()

	targetDirectory := outputDirectory
	if targetDirectory == "" {
		targetDirectory = fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}
	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		if err := os.Mkdir(targetDirectory, 0o755); err != nil {
			log.Fatal("Failed to create target directory")
		}
	}

	payload.SetConcurrency(concurrency)
	fmt.Printf("Number of workers: %d\n", payload.GetConcurrency())

	if partitions != "" {
		if err := payload.ExtractSelected(targetDirectory, strings.Split(partitions, ",")); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := payload.ExtractAll(targetDirectory); err != nil {
			log.Fatal(err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] [inputfile]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}