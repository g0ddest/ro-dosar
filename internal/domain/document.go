package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DocumentNumber is a value object representing a document number
type DocumentNumber struct {
	Number   int    // e.g., 10435
	Category string // e.g., A, RD, P
	Year     int    // e.g., 2025
}

var (
	// Pattern to match document numbers like "10435/A/2025" or "10435\A\2025"
	documentNumberPattern = regexp.MustCompile(`^\s*(\d+)\s*[/\\]\s*([A-Za-z]+)\s*[/\\]\s*(\d{4})\s*$`)
)

// ParseDocumentNumber parses a string into a DocumentNumber
// Normalizes: removes spaces, replaces \ with /, converts to uniform format
func ParseDocumentNumber(s string) (DocumentNumber, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\", "/")

	matches := documentNumberPattern.FindStringSubmatch(s)
	if matches == nil {
		return DocumentNumber{}, fmt.Errorf("invalid document number format: %s", s)
	}

	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return DocumentNumber{}, fmt.Errorf("invalid number part: %s", matches[1])
	}

	year, err := strconv.Atoi(matches[3])
	if err != nil {
		return DocumentNumber{}, fmt.Errorf("invalid year part: %s", matches[3])
	}

	return DocumentNumber{
		Number:   number,
		Category: strings.ToUpper(matches[2]),
		Year:     year,
	}, nil
}

// String returns the normalized string representation
func (d DocumentNumber) String() string {
	return fmt.Sprintf("%d/%s/%d", d.Number, d.Category, d.Year)
}

// IsZero checks if the DocumentNumber is empty/uninitialized
func (d DocumentNumber) IsZero() bool {
	return d.Number == 0 && d.Category == "" && d.Year == 0
}

// Equals checks equality with another DocumentNumber
func (d DocumentNumber) Equals(other DocumentNumber) bool {
	return d.Number == other.Number &&
		d.Category == other.Category &&
		d.Year == other.Year
}

// Document represents a citizenship application document
type Document struct {
	DocumentNumber DocumentNumber
	RegisteredAt   time.Time       // DATA ÎNREGISTRĂRII
	Category       Category        // ART_8, ART_11, etc.
	Term           *time.Time      // TERMEN (optional)
	SolutionNumber *DocumentNumber // SOLUTIE (optional, format 215/P/2021)
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewDocument creates a new Document entity
func NewDocument(docNum DocumentNumber, registeredAt time.Time, category Category) *Document {
	now := time.Now()
	return &Document{
		DocumentNumber: docNum,
		RegisteredAt:   registeredAt,
		Category:       category,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// SetTerm sets the term date
func (d *Document) SetTerm(term time.Time) {
	d.Term = &term
	d.UpdatedAt = time.Now()
}

// SetSolutionNumber sets the solution number
func (d *Document) SetSolutionNumber(solutionNum DocumentNumber) {
	d.SolutionNumber = &solutionNum
	d.UpdatedAt = time.Now()
}

// HasChanges checks if the document has changes compared to another
func (d *Document) HasChanges(other *Document) bool {
	if other == nil {
		return true
	}

	if !d.DocumentNumber.Equals(other.DocumentNumber) {
		return true
	}

	if !d.RegisteredAt.Equal(other.RegisteredAt) {
		return true
	}

	if d.Category != other.Category {
		return true
	}

	// Compare Term
	if (d.Term == nil) != (other.Term == nil) {
		return true
	}
	if d.Term != nil && other.Term != nil && !d.Term.Equal(*other.Term) {
		return true
	}

	// Compare SolutionNumber
	if (d.SolutionNumber == nil) != (other.SolutionNumber == nil) {
		return true
	}
	if d.SolutionNumber != nil && other.SolutionNumber != nil && !d.SolutionNumber.Equals(*other.SolutionNumber) {
		return true
	}

	return false
}
