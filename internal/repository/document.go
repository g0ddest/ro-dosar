package repository

import (
	"context"
	"time"

	"ro-dosar/internal/domain"
)

// DocumentRepository defines the interface for document persistence
type DocumentRepository interface {
	// GetByNumber retrieves a document by its document number
	GetByNumber(ctx context.Context, docNum domain.DocumentNumber) (*domain.Document, error)

	// Save creates or updates a document
	Save(ctx context.Context, doc *domain.Document) error

	// Delete removes a document
	Delete(ctx context.Context, docNum domain.DocumentNumber) error

	// List retrieves documents with optional filtering
	List(ctx context.Context, filter DocumentFilter) ([]*domain.Document, error)
}

// DocumentFilter represents filtering options for listing documents
type DocumentFilter struct {
	Category *domain.Category
	Year     *int
	Limit    int
	Offset   int
}

// AuditAction represents the type of audit action
type AuditAction string

const (
	AuditActionCreate AuditAction = "CREATE"
	AuditActionUpdate AuditAction = "UPDATE"
	AuditActionDelete AuditAction = "DELETE"
)

// ChangeEvent is one audit-log transition relevant to the dossier timeline
type ChangeEvent struct {
	NoticedAt   time.Time
	OldTerm     *string
	NewTerm     *string
	OldSolution *string
	NewSolution *string
}

// DocumentAuditRepository defines the interface for document audit logging
type DocumentAuditRepository interface {
	// Log records an audit entry for a document change
	Log(ctx context.Context, docNum domain.DocumentNumber, action AuditAction, oldState, newState *domain.Document) error

	// GetHistory retrieves the audit history for a document
	GetHistory(ctx context.Context, docNum domain.DocumentNumber) ([]DocumentAuditEntry, error)

	// GetChangeEvents returns the document's audit transitions in
	// chronological order, for timeline assembly
	GetChangeEvents(ctx context.Context, docNum domain.DocumentNumber) ([]ChangeEvent, error)
}

// DocumentAuditEntry represents an audit log entry
type DocumentAuditEntry struct {
	ID             int
	DocumentNumber string
	Action         AuditAction
	OldState       *domain.Document
	NewState       *domain.Document
	CreatedAt      string
}
