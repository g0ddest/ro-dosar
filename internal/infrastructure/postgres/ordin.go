package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// OrdinRepository implements repository.OrdinRepository using PostgreSQL
type OrdinRepository struct {
	db *DB
}

// NewOrdinRepository creates a new PostgreSQL ordin repository
func NewOrdinRepository(db *DB) *OrdinRepository {
	return &OrdinRepository{db: db}
}

// SaveBatch upserts ordins by URL
func (r *OrdinRepository) SaveBatch(ctx context.Context, ordins []repository.Ordin) error {
	query := `
		INSERT INTO ordine (url, ordin_number, letter, ordin_date, source_page, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (url) DO UPDATE SET
			ordin_number = EXCLUDED.ordin_number,
			letter = EXCLUDED.letter,
			ordin_date = EXCLUDED.ordin_date,
			source_page = EXCLUDED.source_page,
			updated_at = NOW()
	`

	for _, o := range ordins {
		if _, err := r.db.Pool.Exec(ctx, query, o.URL, o.Number, o.Letter, o.Date, o.SourcePage); err != nil {
			return err
		}
	}
	return nil
}

// GetBySolution finds the ordin matching a solution number's parts
func (r *OrdinRepository) GetBySolution(ctx context.Context, number int, letter string, year int) (*repository.Ordin, error) {
	query := `
		SELECT url, ordin_number, letter, ordin_date, source_page
		FROM ordine
		WHERE ordin_number = $1 AND letter = $2
		  AND EXTRACT(YEAR FROM ordin_date)::int = $3
		ORDER BY ordin_date
		LIMIT 1
	`

	var o repository.Ordin
	err := r.db.Pool.QueryRow(ctx, query, number, letter, year).Scan(
		&o.URL, &o.Number, &o.Letter, &o.Date, &o.SourcePage,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrdinNotFound
		}
		return nil, err
	}
	return &o, nil
}
