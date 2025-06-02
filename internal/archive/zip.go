package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
)

// ExtractPayloadFromZip extracts payload.bin from a ZIP archive to a temporary file
func ExtractPayloadFromZip(filename string) (string, error) {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		return "", fmt.Errorf("not a valid zip archive: %s", filename)
	}
	defer zipReader.Close()

	for _, file := range zipReader.Reader.File {
		if file.Name == "payload.bin" && file.UncompressedSize64 > 0 {
			zippedFile, err := file.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read zipped file: %s", file.Name)
			}
			defer zippedFile.Close()

			tempfile, err := os.CreateTemp(os.TempDir(), "payload_*.bin")
			if err != nil {
				return "", fmt.Errorf("failed to create temp file: %v", err)
			}
			defer tempfile.Close()

			_, err = io.Copy(tempfile, zippedFile)
			if err != nil {
				os.Remove(tempfile.Name())
				return "", err
			}

			return tempfile.Name(), nil
		}
	}

	return "", fmt.Errorf("payload.bin not found in archive")
}

// CleanupTempFile removes a temporary file and logs any errors
func CleanupTempFile(filename string) {
	if err := os.Remove(filename); err != nil {
		log.Printf("Warning: failed to remove temp file %s: %v", filename, err)
	}
}