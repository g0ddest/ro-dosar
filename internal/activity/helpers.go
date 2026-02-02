package activity

import (
	"bytes"
	"io"
	"time"
)

// bytesReader wraps bytes.Reader to implement io.Reader
type bytesReader struct {
	*bytes.Reader
}

// newBytesReader creates a new bytes reader
func newBytesReader(data []byte) io.Reader {
	return &bytesReader{Reader: bytes.NewReader(data)}
}

// parseDate parses a date string in format "2006-01-02"
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// parseTime parses a time string in format "15:04"
func parseTime(s string) (time.Time, error) {
	return time.Parse("15:04", s)
}
