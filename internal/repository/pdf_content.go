package repository

import "context"

// PDFContentRepository defines the interface for PDF content storage
type PDFContentRepository interface {
	// Save stores PDF content by hash
	Save(ctx context.Context, hash string, content []byte) error

	// Get retrieves PDF content by hash
	Get(ctx context.Context, hash string) ([]byte, error)

	// Delete removes PDF content by hash
	Delete(ctx context.Context, hash string) error
}
