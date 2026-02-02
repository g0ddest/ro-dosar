package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// DocumentRepository implements repository.DocumentRepository using PostgreSQL
type DocumentRepository struct {
	db *DB
}

// NewDocumentRepository creates a new PostgreSQL document repository
func NewDocumentRepository(db *DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// GetByNumber retrieves a document by its document number
func (r *DocumentRepository) GetByNumber(ctx context.Context, docNum domain.DocumentNumber) (*domain.Document, error) {
	query := `
		SELECT document_number, registered_at, category, term, solution_number, created_at, updated_at
		FROM documents
		WHERE document_number = $1
	`

	var (
		docNumStr      string
		registeredAt   time.Time
		category       string
		term           *time.Time
		solutionNumber *string
		createdAt      time.Time
		updatedAt      time.Time
	)

	err := r.db.Pool.QueryRow(ctx, query, docNum.String()).Scan(
		&docNumStr,
		&registeredAt,
		&category,
		&term,
		&solutionNumber,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDocumentNotFound
		}
		return nil, err
	}

	parsedDocNum, err := domain.ParseDocumentNumber(docNumStr)
	if err != nil {
		return nil, err
	}

	doc := &domain.Document{
		DocumentNumber: parsedDocNum,
		RegisteredAt:   registeredAt,
		Category:       domain.Category(category),
		Term:           term,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	if solutionNumber != nil {
		solNum, err := domain.ParseDocumentNumber(*solutionNumber)
		if err == nil {
			doc.SolutionNumber = &solNum
		}
	}

	return doc, nil
}

// Save creates or updates a document
func (r *DocumentRepository) Save(ctx context.Context, doc *domain.Document) error {
	var solutionNumber *string
	if doc.SolutionNumber != nil {
		s := doc.SolutionNumber.String()
		solutionNumber = &s
	}

	query := `
		INSERT INTO documents (document_number, registered_at, category, term, solution_number, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (document_number) DO UPDATE SET
			registered_at = EXCLUDED.registered_at,
			category = EXCLUDED.category,
			term = EXCLUDED.term,
			solution_number = EXCLUDED.solution_number,
			updated_at = NOW()
	`

	_, err := r.db.Pool.Exec(ctx, query,
		doc.DocumentNumber.String(),
		doc.RegisteredAt,
		doc.Category.String(),
		doc.Term,
		solutionNumber,
		doc.CreatedAt,
		doc.UpdatedAt,
	)

	return err
}

// Delete removes a document
func (r *DocumentRepository) Delete(ctx context.Context, docNum domain.DocumentNumber) error {
	query := `DELETE FROM documents WHERE document_number = $1`
	_, err := r.db.Pool.Exec(ctx, query, docNum.String())
	return err
}

// List retrieves documents with optional filtering
func (r *DocumentRepository) List(ctx context.Context, filter repository.DocumentFilter) ([]*domain.Document, error) {
	query := `
		SELECT document_number, registered_at, category, term, solution_number, created_at, updated_at
		FROM documents
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if filter.Category != nil {
		query += ` AND category = $` + string(rune('0'+argIndex))
		args = append(args, filter.Category.String())
		argIndex++
	}

	if filter.Year != nil {
		query += ` AND document_number LIKE $` + string(rune('0'+argIndex))
		args = append(args, "%/"+string(rune('0'+*filter.Year/1000))+"%")
		argIndex++
	}

	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += ` LIMIT $` + string(rune('0'+argIndex))
		args = append(args, filter.Limit)
		argIndex++
	}

	if filter.Offset > 0 {
		query += ` OFFSET $` + string(rune('0'+argIndex))
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var documents []*domain.Document
	for rows.Next() {
		var (
			docNumStr      string
			registeredAt   time.Time
			category       string
			term           *time.Time
			solutionNumber *string
			createdAt      time.Time
			updatedAt      time.Time
		)

		if err := rows.Scan(&docNumStr, &registeredAt, &category, &term, &solutionNumber, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		parsedDocNum, err := domain.ParseDocumentNumber(docNumStr)
		if err != nil {
			continue
		}

		doc := &domain.Document{
			DocumentNumber: parsedDocNum,
			RegisteredAt:   registeredAt,
			Category:       domain.Category(category),
			Term:           term,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		}

		if solutionNumber != nil {
			solNum, err := domain.ParseDocumentNumber(*solutionNumber)
			if err == nil {
				doc.SolutionNumber = &solNum
			}
		}

		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// DocumentAuditRepository implements repository.DocumentAuditRepository using PostgreSQL
type DocumentAuditRepository struct {
	db *DB
}

// NewDocumentAuditRepository creates a new PostgreSQL document audit repository
func NewDocumentAuditRepository(db *DB) *DocumentAuditRepository {
	return &DocumentAuditRepository{db: db}
}

// Log records an audit entry for a document change
func (r *DocumentAuditRepository) Log(ctx context.Context, docNum domain.DocumentNumber, action repository.AuditAction, oldState, newState *domain.Document) error {
	var oldStateJSON, newStateJSON []byte
	var err error

	if oldState != nil {
		oldStateJSON, err = json.Marshal(documentToMap(oldState))
		if err != nil {
			return err
		}
	}

	if newState != nil {
		newStateJSON, err = json.Marshal(documentToMap(newState))
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO document_audit_log (document_number, action, old_state, new_state)
		VALUES ($1, $2, $3, $4)
	`

	_, err = r.db.Pool.Exec(ctx, query, docNum.String(), string(action), oldStateJSON, newStateJSON)
	return err
}

// GetHistory retrieves the audit history for a document
func (r *DocumentAuditRepository) GetHistory(ctx context.Context, docNum domain.DocumentNumber) ([]repository.DocumentAuditEntry, error) {
	query := `
		SELECT id, document_number, action, old_state, new_state, created_at
		FROM document_audit_log
		WHERE document_number = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, docNum.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []repository.DocumentAuditEntry
	for rows.Next() {
		var (
			id           int
			docNumStr    string
			action       string
			oldStateJSON []byte
			newStateJSON []byte
			createdAt    time.Time
		)

		if err := rows.Scan(&id, &docNumStr, &action, &oldStateJSON, &newStateJSON, &createdAt); err != nil {
			return nil, err
		}

		entry := repository.DocumentAuditEntry{
			ID:             id,
			DocumentNumber: docNumStr,
			Action:         repository.AuditAction(action),
			CreatedAt:      createdAt.Format(time.RFC3339),
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// documentToMap converts a Document to a map for JSON serialization
func documentToMap(doc *domain.Document) map[string]interface{} {
	m := map[string]interface{}{
		"document_number": doc.DocumentNumber.String(),
		"registered_at":   doc.RegisteredAt.Format("2006-01-02"),
		"category":        doc.Category.String(),
	}

	if doc.Term != nil {
		m["term"] = doc.Term.Format("2006-01-02")
	}

	if doc.SolutionNumber != nil {
		m["solution_number"] = doc.SolutionNumber.String()
	}

	return m
}
