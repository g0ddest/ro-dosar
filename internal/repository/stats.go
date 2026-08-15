package repository

import (
	"context"
	"time"
)

// CategoryYearStats is an aggregate over documents for one category and registration year
type CategoryYearStats struct {
	Category string
	Year     int
	Total    int
	Solved   int
	WithTerm int
}

// CategoryYearActivity is recent audit-derived activity for one category and registration year
type CategoryYearActivity struct {
	Category string
	Year     int
	Solved   int
	TermsSet int
}

// CohortCell is one (registration year, solution year) aggregation cell:
// how many of the category's dossiers registered in RegYear received their
// solution in SolYear, with percentiles of the numeric document-number part
type CohortCell struct {
	Category string
	RegYear  int
	SolYear  int
	Count    int
	P50      int
	P90      int
}

// StatsRepository defines the interface for processing statistics queries
type StatsRepository interface {
	// GetYearlyStats returns per-category, per-registration-year document counts
	GetYearlyStats(ctx context.Context) ([]CategoryYearStats, error)

	// GetRecentActivity returns per-category, per-registration-year counts of
	// solutions appeared and terms changed since the given time
	GetRecentActivity(ctx context.Context, since time.Time) ([]CategoryYearActivity, error)

	// CountAheadInQueue counts unsolved documents of the category registered
	// before the given document: earlier year, or same year with a smaller
	// document number (numbers are sequential within a year)
	CountAheadInQueue(ctx context.Context, category string, year, number int) (int, error)

	// CountSolvedInYear counts the category's documents whose solution_number
	// year segment (e.g. 215/P/2021 -> 2021) equals the given year
	CountSolvedInYear(ctx context.Context, category string, solutionYear int) (int, error)

	// GetCohortMatrix returns solved-dossier counts and number percentiles
	// grouped by category, registration year and solution year
	GetCohortMatrix(ctx context.Context) ([]CohortCell, error)
}
