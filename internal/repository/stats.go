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
}
