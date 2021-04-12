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

	list := flag.Bool("l", false, "Show list of partitions in payload.bin")
	partitions := flag.String("p", "", "Dump only selected partitions")
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
	}
	filename := flag.Arg(0)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Fatalf("File does not exist: %s\n", filename)
	}

	payloadBin := filename
	if strings.HasSuffix(filename, ".zip") {
		fmt.Println("Please wait while extracting payload.bin from the archive.")
		payloadBin = extractPayloadBin(filename)
		if payloadBin == "" {
			log.Fatal("Failed to extract payload.bin from the archive.")
		}
	}
	fmt.Printf("payload.bin: %s\n", payloadBin)

	payload := NewPayload(payloadBin)
	if err := payload.Open(); err != nil {
		log.Fatal(err)
	}
	payload.Init()

	if *list {
		return
	}

	now := time.Now()
	targetDirectory := fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	if err := os.Mkdir(targetDirectory, 0755); err != nil {
		log.Fatal("Failed to create target directory")
	}
	if *partitions != "" {
		if err := payload.ExtractSelected(targetDirectory, strings.Split(*partitions, ",")); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := payload.ExtractAll(targetDirectory); err != nil {
			log.Fatal(err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [inputfile]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}
