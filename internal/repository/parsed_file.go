package repository

import (
	"context"

	"ro-dosar/internal/domain"
)

// ParsedFileRepository defines the interface for parsed file persistence
type ParsedFileRepository interface {
	// GetByURI retrieves a parsed file by its URI
	GetByURI(ctx context.Context, uri string) (*domain.ParsedFile, error)

	// Save creates or updates a parsed file record
	Save(ctx context.Context, file *domain.ParsedFile) error

	// Delete removes a parsed file record
	Delete(ctx context.Context, uri string) error

	// List retrieves parsed files with optional filtering
	List(ctx context.Context, filter ParsedFileFilter) ([]*domain.ParsedFile, error)
}

// ParsedFileFilter represents filtering options for listing parsed files
type ParsedFileFilter struct {
	Category *domain.Category
	Type     *domain.FileType
	Limit    int
	Offset   int
}
