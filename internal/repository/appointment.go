package repository

import (
	"context"

	"ro-dosar/internal/domain"
)

// AppointmentRepository defines the interface for appointment persistence
type AppointmentRepository interface {
	// GetByID retrieves an appointment by its ID
	GetByID(ctx context.Context, id int) (*domain.Appointment, error)

	// GetByDocumentNumber retrieves all appointments for a document
	GetByDocumentNumber(ctx context.Context, docNum domain.DocumentNumber) ([]*domain.Appointment, error)

	// GetByDocumentAndType retrieves appointments for a document filtered by type
	GetByDocumentAndType(ctx context.Context, docNum domain.DocumentNumber, appointmentType domain.AppointmentType) ([]*domain.Appointment, error)

	// Save creates or updates an appointment (upsert by document_number, date, type)
	Save(ctx context.Context, appointment *domain.Appointment) error

	// Delete removes an appointment
	Delete(ctx context.Context, id int) error
}
