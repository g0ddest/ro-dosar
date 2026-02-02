package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PDFContentRepository implements repository.PDFContentRepository using PostgreSQL
type PDFContentRepository struct {
	db *DB
}

// NewPDFContentRepository creates a new PDF content repository
func NewPDFContentRepository(db *DB) *PDFContentRepository {
	return &PDFContentRepository{db: db}
}

// Save stores PDF content by hash
func (r *PDFContentRepository) Save(ctx context.Context, hash string, content []byte) error {
	query := `
		INSERT INTO pdf_content (hash, content)
		VALUES ($1, $2)
		ON CONFLICT (hash) DO UPDATE SET content = EXCLUDED.content
	`
	_, err := r.db.Pool.Exec(ctx, query, hash, content)
	if err != nil {
		return fmt.Errorf("failed to save pdf content: %w", err)
	}
	return nil
}

// Get retrieves PDF content by hash
func (r *PDFContentRepository) Get(ctx context.Context, hash string) ([]byte, error) {
	query := `SELECT content FROM pdf_content WHERE hash = $1`
	var content []byte
	err := r.db.Pool.QueryRow(ctx, query, hash).Scan(&content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("pdf content not found: %s", hash)
		}
		return nil, fmt.Errorf("failed to get pdf content: %w", err)
	}
	return content, nil
}

// Delete removes PDF content by hash
func (r *PDFContentRepository) Delete(ctx context.Context, hash string) error {
	query := `DELETE FROM pdf_content WHERE hash = $1`
	_, err := r.db.Pool.Exec(ctx, query, hash)
	if err != nil {
		return fmt.Errorf("failed to delete pdf content: %w", err)
	}
	return nil
}
