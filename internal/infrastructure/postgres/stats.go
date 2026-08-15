package postgres

import (
	"context"
	"strconv"
	"time"

	"ro-dosar/internal/repository"
)

// StatsRepository implements repository.StatsRepository using PostgreSQL
type StatsRepository struct {
	db *DB
}

// NewStatsRepository creates a new PostgreSQL stats repository
func NewStatsRepository(db *DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// GetYearlyStats returns per-category, per-registration-year document counts
func (r *StatsRepository) GetYearlyStats(ctx context.Context) ([]repository.CategoryYearStats, error) {
	query := `
		SELECT category,
		       EXTRACT(YEAR FROM registered_at)::int AS year,
		       COUNT(*)::int               AS total,
		       COUNT(solution_number)::int AS solved,
		       COUNT(term)::int            AS with_term
		FROM documents
		GROUP BY 1, 2
		ORDER BY 1, 2
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []repository.CategoryYearStats
	for rows.Next() {
		var s repository.CategoryYearStats
		if err := rows.Scan(&s.Category, &s.Year, &s.Total, &s.Solved, &s.WithTerm); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetRecentActivity returns per-category, per-registration-year counts of
// solutions appeared and terms changed since the given time.
// Only UPDATE audit entries count: the initial parse creates all records at
// once and would flood the activity signal via CREATE entries.
func (r *StatsRepository) GetRecentActivity(ctx context.Context, since time.Time) ([]repository.CategoryYearActivity, error) {
	query := `
		SELECT new_state->>'category' AS category,
		       EXTRACT(YEAR FROM (new_state->>'registered_at')::date)::int AS year,
		       (COUNT(*) FILTER (WHERE old_state->>'solution_number' IS NULL
		                           AND new_state->>'solution_number' IS NOT NULL))::int AS solved,
		       (COUNT(*) FILTER (WHERE new_state->>'term' IS NOT NULL
		                           AND old_state->>'term' IS DISTINCT FROM
		                               new_state->>'term'))::int                        AS terms_set
		FROM document_audit_log
		WHERE action = 'UPDATE'
		  AND created_at >= $1
		  AND new_state->>'registered_at' IS NOT NULL
		GROUP BY 1, 2
	`

	rows, err := r.db.Pool.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activity []repository.CategoryYearActivity
	for rows.Next() {
		var a repository.CategoryYearActivity
		if err := rows.Scan(&a.Category, &a.Year, &a.Solved, &a.TermsSet); err != nil {
			return nil, err
		}
		activity = append(activity, a)
	}

	return activity, rows.Err()
}

// CountAheadInQueue counts unsolved documents of the category registered
// before the given document (earlier year, or same year with a smaller number)
func (r *StatsRepository) CountAheadInQueue(ctx context.Context, category string, year, number int) (int, error) {
	query := `
		SELECT COUNT(*)::int
		FROM documents
		WHERE category = $1
		  AND solution_number IS NULL
		  AND (EXTRACT(YEAR FROM registered_at)::int < $2
		       OR (EXTRACT(YEAR FROM registered_at)::int = $2
		           AND split_part(document_number, '/', 1)::int < $3))
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, category, year, number).Scan(&count)
	return count, err
}

// CountSolvedInYear counts the category's documents whose solution_number
// year segment equals the given year
func (r *StatsRepository) CountSolvedInYear(ctx context.Context, category string, solutionYear int) (int, error) {
	query := `
		SELECT COUNT(*)::int
		FROM documents
		WHERE category = $1
		  AND solution_number IS NOT NULL
		  AND split_part(solution_number, '/', 3) = $2
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, category, strconv.Itoa(solutionYear)).Scan(&count)
	return count, err
}

// GetCohortMatrix returns solved-dossier counts and number percentiles
// grouped by category, registration year and solution year
func (r *StatsRepository) GetCohortMatrix(ctx context.Context) ([]repository.CohortCell, error) {
	query := `
		SELECT category,
		       EXTRACT(YEAR FROM registered_at)::int AS reg_year,
		       split_part(solution_number, '/', 3)::int AS sol_year,
		       COUNT(*)::int AS cnt,
		       (percentile_cont(0.5) WITHIN GROUP
		         (ORDER BY split_part(document_number, '/', 1)::int))::int AS p50,
		       (percentile_cont(0.9) WITHIN GROUP
		         (ORDER BY split_part(document_number, '/', 1)::int))::int AS p90
		FROM documents
		WHERE solution_number IS NOT NULL
		  AND split_part(solution_number, '/', 3) ~ '^[0-9]{4}$'
		GROUP BY 1, 2, 3
		ORDER BY 1, 2, 3
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []repository.CohortCell
	for rows.Next() {
		var c repository.CohortCell
		if err := rows.Scan(&c.Category, &c.RegYear, &c.SolYear, &c.Count, &c.P50, &c.P90); err != nil {
			return nil, err
		}
		cells = append(cells, c)
	}

	return cells, rows.Err()
}
