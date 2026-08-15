package repository

import (
	"context"
	"time"
)

// Ordin is one published ANC ordin indexed from the Ordine listing pages
type Ordin struct {
	URL        string
	Number     int
	Letter     string
	Date       time.Time
	SourcePage string
}

// OrdinRepository defines the interface for the ordin index
type OrdinRepository interface {
	// SaveBatch upserts ordins by URL
	SaveBatch(ctx context.Context, ordins []Ordin) error

	// GetBySolution finds the ordin matching a solution number's parts;
	// the year is matched against the ordin date's year
	GetBySolution(ctx context.Context, number int, letter string, year int) (*Ordin, error)
}
