package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// OathRepository implements repository.OathRepository using PostgreSQL
type OathRepository struct {
	db *DB
}

// NewOathRepository creates a new PostgreSQL oath repository
func NewOathRepository(db *DB) *OathRepository {
	return &OathRepository{db: db}
}

// SaveBatch inserts entries, ignoring already-known rows
func (r *OathRepository) SaveBatch(ctx context.Context, entries []repository.OathEntryRecord) error {
	query := `
		INSERT INTO oath_schedule (doc_number, doc_year, oath_date, oath_time, list_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (doc_number, doc_year, oath_date) DO NOTHING
	`

	for _, e := range entries {
		if _, err := r.db.Pool.Exec(ctx, query, e.Number, e.Year, e.Date, e.Time, e.ListURL); err != nil {
			return err
		}
	}
	return nil
}

// GetByDossier returns the latest scheduled oath for the dossier's number and year
func (r *OathRepository) GetByDossier(ctx context.Context, number, year int) (*repository.OathEntryRecord, error) {
	query := `
		SELECT doc_number, doc_year, oath_date, to_char(oath_time, 'HH24:MI'), list_url
		FROM oath_schedule
		WHERE doc_number = $1 AND doc_year = $2
		ORDER BY oath_date DESC
		LIMIT 1
	`

	var e repository.OathEntryRecord
	err := r.db.Pool.QueryRow(ctx, query, number, year).Scan(&e.Number, &e.Year, &e.Date, &e.Time, &e.ListURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOathNotFound
		}
		return nil, err
	}
	return &e, nil
}
