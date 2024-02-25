package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

func extractPayloadBin(filename string) string {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		log.Fatalf("Not a valid zip archive: %s\n", filename)
	}
	defer zipReader.Close()

	for _, file := range zipReader.Reader.File {
		if file.Name == "payload.bin" && file.UncompressedSize64 > 0 {
			zippedFile, err := file.Open()
			if err != nil {
				log.Fatalf("Failed to read zipped file: %s\n", file.Name)
			}

			tempfile, err := ioutil.TempFile(os.TempDir(), "payload_*.bin")
			if err != nil {
				log.Fatalf("Failed to create a temp file located at %s\n", tempfile.Name())
			}
			defer tempfile.Close()

			_, err = io.Copy(tempfile, zippedFile)
			if err != nil {
				log.Fatal(err)
			}

			return tempfile.Name()
		}
	}

	return ""
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		list            bool
		partitions      string
		outputDirectory string
		inputDirectory  string
		concurrency     int
	)

	flag.IntVar(&concurrency, "c", 4, "Number of multiple workers to extract (shorthand)")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of multiple workers to extract")
	flag.BoolVar(&list, "l", false, "Show list of partitions in payload.bin (shorthand)")
	flag.BoolVar(&list, "list", false, "Show list of partitions in payload.bin")
	flag.StringVar(&outputDirectory, "o", "", "Set output directory (shorthand)")
	flag.StringVar(&outputDirectory, "output", "", "Set output directory")
	flag.StringVar(&inputDirectory, "d", "", "Set input directory for delta payload (shorthand)")
	flag.StringVar(&inputDirectory, "delta", "", "Set input directory for delta payload")
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

	log.Printf("Delta: %s, partitions: %s\n", inputDirectory, partitions)

	payloadBin := filename
	if strings.HasSuffix(filename, ".zip") {
		fmt.Println("Please wait while extracting payload.bin from the archive.")
		payloadBin = extractPayloadBin(filename)
		if payloadBin == "" {
			log.Fatal("Failed to extract payload.bin from the archive.")
		} else {
			defer os.Remove(payloadBin)
		}
	}
	fmt.Printf("payload.bin: %s\n", payloadBin)

	payload := NewPayload(payloadBin)
	if err := payload.Open(); err != nil {
		log.Fatal(err)
	}
	payload.Init()

	if list {
		return
	}

	now := time.Now()

	var targetDirectory = outputDirectory
	if targetDirectory == "" {
		targetDirectory = fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}
	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		if err := os.Mkdir(targetDirectory, 0755); err != nil {
			log.Fatal("Failed to create target directory")
		}
	}
	var sourceDirectory = inputDirectory
	payload.SetConcurrency(concurrency)
	fmt.Printf("Number of workers: %d\n", payload.GetConcurrency())

	if partitions != "" {
		if err := payload.ExtractSelected(sourceDirectory, targetDirectory, strings.Split(partitions, ",")); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := payload.ExtractAll(sourceDirectory, targetDirectory); err != nil {
			log.Fatal(err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] [inputfile]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}
