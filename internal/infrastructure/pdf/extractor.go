package pdf

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// TextExtractor extracts text from PDF files using pdftotext
type TextExtractor struct{}

// NewTextExtractor creates a new PDF text extractor
func NewTextExtractor() *TextExtractor {
	return &TextExtractor{}
}

// ExtractText extracts text from PDF data with layout preserved
func (e *TextExtractor) ExtractText(data []byte) (string, error) {
	// Write PDF data to temporary file
	tmpFile, err := os.CreateTemp("", "pdf-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Run pdftotext with layout option
	cmd := exec.Command("pdftotext", "-layout", tmpFile.Name(), "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExtractTextRaw extracts text from PDF data without layout preservation
func (e *TextExtractor) ExtractTextRaw(data []byte) (string, error) {
	// Write PDF data to temporary file
	tmpFile, err := os.CreateTemp("", "pdf-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Run pdftotext without layout option
	cmd := exec.Command("pdftotext", tmpFile.Name(), "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
