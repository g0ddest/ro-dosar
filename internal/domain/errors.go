package domain

import "errors"

// Domain errors
var (
	ErrDocumentNotFound      = errors.New("document not found")
	ErrAppointmentNotFound   = errors.New("appointment not found")
	ErrParsedFileNotFound    = errors.New("parsed file not found")
	ErrInvalidDocumentNumber = errors.New("invalid document number")
	ErrInvalidCategory       = errors.New("invalid category")
	ErrInvalidFileType       = errors.New("invalid file type")
	ErrRemoteFileNotFound    = errors.New("remote file not found (HTTP 404)")
)
