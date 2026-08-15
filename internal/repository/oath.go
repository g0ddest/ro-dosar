package repository

import (
	"context"
	"time"
)

// OathEntryRecord is one scheduled-oath fact for a dossier (no category —
// the published lists carry number/year only)
type OathEntryRecord struct {
	Number  int
	Year    int
	Date    time.Time
	Time    *string
	ListURL string
}

// OathRepository defines the interface for the oath schedule index
type OathRepository interface {
	// SaveBatch inserts entries, ignoring already-known (number, year, date) rows
	SaveBatch(ctx context.Context, entries []OathEntryRecord) error

	// GetByDossier returns the latest scheduled oath for the dossier's
	// number and year, or domain.ErrOathNotFound
	GetByDossier(ctx context.Context, number, year int) (*OathEntryRecord, error)
}
