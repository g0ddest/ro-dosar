package domain

import (
	"time"
)

// FileType represents the type of parsed file
type FileType string

const (
	FileTypeApplication           FileType = "APPLICATION"
	FileTypeAppointmentInvitation FileType = "APPOINTMENT_INVITATION"
	FileTypeAppointmentResult     FileType = "APPOINTMENT_RESULT"
)

// IsValid checks if the file type is valid
func (t FileType) IsValid() bool {
	switch t {
	case FileTypeApplication, FileTypeAppointmentInvitation, FileTypeAppointmentResult:
		return true
	}
	return false
}

// String returns the string representation
func (t FileType) String() string {
	return string(t)
}

// FileStatus represents the status of a parsed file
type FileStatus string

const (
	FileStatusParsed   FileStatus = "PARSED"
	FileStatusNotFound FileStatus = "NOT_FOUND"
)

// String returns the string representation
func (s FileStatus) String() string {
	return string(s)
}

// ParsedFile represents a parsed PDF file record
type ParsedFile struct {
	URI       string
	Hash      string
	Category  Category
	Type      FileType
	Status    FileStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewParsedFile creates a new ParsedFile entity
func NewParsedFile(uri, hash string, category Category, fileType FileType) *ParsedFile {
	now := time.Now()
	return &ParsedFile{
		URI:       uri,
		Hash:      hash,
		Category:  category,
		Type:      fileType,
		Status:    FileStatusParsed,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewNotFoundFile creates a ParsedFile record for a file that was not found
func NewNotFoundFile(uri string, category Category, fileType FileType) *ParsedFile {
	now := time.Now()
	return &ParsedFile{
		URI:       uri,
		Hash:      "",
		Category:  category,
		Type:      fileType,
		Status:    FileStatusNotFound,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// UpdateHash updates the file hash
func (p *ParsedFile) UpdateHash(hash string) {
	p.Hash = hash
	p.UpdatedAt = time.Now()
}

// HasChanged checks if the file has changed by comparing hashes
func (p *ParsedFile) HasChanged(newHash string) bool {
	return p.Hash != newHash
}
