package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// ParsedFileRepository implements repository.ParsedFileRepository using PostgreSQL
type ParsedFileRepository struct {
	db *DB
}

// NewParsedFileRepository creates a new PostgreSQL parsed file repository
func NewParsedFileRepository(db *DB) *ParsedFileRepository {
	return &ParsedFileRepository{db: db}
}

// GetByURI retrieves a parsed file by its URI
func (r *ParsedFileRepository) GetByURI(ctx context.Context, uri string) (*domain.ParsedFile, error) {
	query := `
		SELECT uri, hash, category, type, COALESCE(status, 'PARSED') as status, created_at, updated_at
		FROM parsed_files
		WHERE uri = $1
	`

	var (
		fileURI   string
		hash      string
		category  string
		fileType  string
		status    string
		createdAt time.Time
		updatedAt time.Time
	)

	err := r.db.Pool.QueryRow(ctx, query, uri).Scan(
		&fileURI,
		&hash,
		&category,
		&fileType,
		&status,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrParsedFileNotFound
		}
		return nil, err
	}

	return &domain.ParsedFile{
		URI:       fileURI,
		Hash:      hash,
		Category:  domain.Category(category),
		Type:      domain.FileType(fileType),
		Status:    domain.FileStatus(status),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// Save creates or updates a parsed file record
func (r *ParsedFileRepository) Save(ctx context.Context, file *domain.ParsedFile) error {
	query := `
		INSERT INTO parsed_files (uri, hash, category, type, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (uri) DO UPDATE SET
			hash = EXCLUDED.hash,
			category = EXCLUDED.category,
			type = EXCLUDED.type,
			status = EXCLUDED.status,
			updated_at = NOW()
	`

	_, err := r.db.Pool.Exec(ctx, query,
		file.URI,
		file.Hash,
		file.Category.String(),
		file.Type.String(),
		file.Status.String(),
		file.CreatedAt,
		file.UpdatedAt,
	)

	return err
}

// Delete removes a parsed file record
func (r *ParsedFileRepository) Delete(ctx context.Context, uri string) error {
	query := `DELETE FROM parsed_files WHERE uri = $1`
	_, err := r.db.Pool.Exec(ctx, query, uri)
	return err
}

// List retrieves parsed files with optional filtering
func (r *ParsedFileRepository) List(ctx context.Context, filter repository.ParsedFileFilter) ([]*domain.ParsedFile, error) {
	query := `
		SELECT uri, hash, category, type, COALESCE(status, 'PARSED') as status, created_at, updated_at
		FROM parsed_files
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if filter.Category != nil {
		query += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, filter.Category.String())
		argIndex++
	}

	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, filter.Type.String())
		argIndex++
	}

	query += " ORDER BY updated_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*domain.ParsedFile
	for rows.Next() {
		var (
			fileURI   string
			hash      string
			category  string
			fileType  string
			status    string
			createdAt time.Time
			updatedAt time.Time
		)

		if err := rows.Scan(&fileURI, &hash, &category, &fileType, &status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		files = append(files, &domain.ParsedFile{
			URI:       fileURI,
			Hash:      hash,
			Category:  domain.Category(category),
			Type:      domain.FileType(fileType),
			Status:    domain.FileStatus(status),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	return files, rows.Err()
}
