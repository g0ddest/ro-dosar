package domain

import (
	"testing"
	"time"
)

func TestParseDocumentNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    DocumentNumber
		wantErr bool
	}{
		{
			name:  "valid document number with slash",
			input: "10435/A/2025",
			want:  DocumentNumber{Number: 10435, Category: "A", Year: 2025},
		},
		{
			name:  "valid document number with backslash",
			input: "10435\\A\\2025",
			want:  DocumentNumber{Number: 10435, Category: "A", Year: 2025},
		},
		{
			name:  "valid document number with spaces",
			input: "10435 / A / 2025",
			want:  DocumentNumber{Number: 10435, Category: "A", Year: 2025},
		},
		{
			name:  "valid document number RD category",
			input: "12345/RD/2024",
			want:  DocumentNumber{Number: 12345, Category: "RD", Year: 2024},
		},
		{
			name:    "invalid format - missing parts",
			input:   "10435/A",
			wantErr: true,
		},
		{
			name:    "invalid format - no separators",
			input:   "10435A2025",
			wantErr: true,
		},
		{
			name:    "invalid number",
			input:   "abc/A/2025",
			wantErr: true,
		},
		{
			name:    "invalid year",
			input:   "10435/A/abc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDocumentNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDocumentNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDocumentNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentNumberString(t *testing.T) {
	tests := []struct {
		name string
		dn   DocumentNumber
		want string
	}{
		{
			name: "simple document number",
			dn:   DocumentNumber{Number: 10435, Category: "A", Year: 2025},
			want: "10435/A/2025",
		},
		{
			name: "RD category",
			dn:   DocumentNumber{Number: 12345, Category: "RD", Year: 2024},
			want: "12345/RD/2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dn.String(); got != tt.want {
				t.Errorf("DocumentNumber.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDocument(t *testing.T) {
	docNum := DocumentNumber{Number: 10435, Category: "A", Year: 2025}
	registeredAt := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	category := CategoryArt8

	doc := NewDocument(docNum, registeredAt, category)

	if doc.DocumentNumber != docNum {
		t.Errorf("NewDocument() DocumentNumber = %v, want %v", doc.DocumentNumber, docNum)
	}
	if doc.RegisteredAt != registeredAt {
		t.Errorf("NewDocument() RegisteredAt = %v, want %v", doc.RegisteredAt, registeredAt)
	}
	if doc.Category != category {
		t.Errorf("NewDocument() Category = %v, want %v", doc.Category, category)
	}
	if doc.Term != nil {
		t.Errorf("NewDocument() Term should be nil")
	}
	if doc.SolutionNumber != nil {
		t.Errorf("NewDocument() SolutionNumber should be nil")
	}
}

func TestDocumentSetTerm(t *testing.T) {
	docNum := DocumentNumber{Number: 10435, Category: "A", Year: 2025}
	registeredAt := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	doc := NewDocument(docNum, registeredAt, CategoryArt8)

	term := time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC)
	doc.SetTerm(term)

	if doc.Term == nil {
		t.Error("SetTerm() Term should not be nil")
	}
	if *doc.Term != term {
		t.Errorf("SetTerm() Term = %v, want %v", *doc.Term, term)
	}
}

func TestDocumentSetSolutionNumber(t *testing.T) {
	docNum := DocumentNumber{Number: 10435, Category: "A", Year: 2025}
	registeredAt := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	doc := NewDocument(docNum, registeredAt, CategoryArt8)

	solNum := DocumentNumber{Number: 215, Category: "P", Year: 2025}
	doc.SetSolutionNumber(solNum)

	if doc.SolutionNumber == nil {
		t.Error("SetSolutionNumber() SolutionNumber should not be nil")
	}
	if *doc.SolutionNumber != solNum {
		t.Errorf("SetSolutionNumber() SolutionNumber = %v, want %v", *doc.SolutionNumber, solNum)
	}
}
