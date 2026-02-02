package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"ro-dosar/internal/domain"
)

// AppointmentRepository implements repository.AppointmentRepository using PostgreSQL
type AppointmentRepository struct {
	db *DB
}

// NewAppointmentRepository creates a new PostgreSQL appointment repository
func NewAppointmentRepository(db *DB) *AppointmentRepository {
	return &AppointmentRepository{db: db}
}

// GetByID retrieves an appointment by its ID
func (r *AppointmentRepository) GetByID(ctx context.Context, id int) (*domain.Appointment, error) {
	query := `
		SELECT id, document_number, date, time, result, type, created_at
		FROM appointments
		WHERE id = $1
	`

	return r.scanAppointment(ctx, query, id)
}

// GetByDocumentNumber retrieves all appointments for a document
func (r *AppointmentRepository) GetByDocumentNumber(ctx context.Context, docNum domain.DocumentNumber) ([]*domain.Appointment, error) {
	query := `
		SELECT id, document_number, date, time, result, type, created_at
		FROM appointments
		WHERE document_number = $1
		ORDER BY date DESC
	`

	return r.scanAppointments(ctx, query, docNum.String())
}

// GetByDocumentAndType retrieves appointments for a document filtered by type
func (r *AppointmentRepository) GetByDocumentAndType(ctx context.Context, docNum domain.DocumentNumber, appointmentType domain.AppointmentType) ([]*domain.Appointment, error) {
	query := `
		SELECT id, document_number, date, time, result, type, created_at
		FROM appointments
		WHERE document_number = $1 AND type = $2
		ORDER BY date DESC
	`

	return r.scanAppointments(ctx, query, docNum.String(), appointmentType.String())
}

// Save creates or updates an appointment (upsert by document_number, date, type)
func (r *AppointmentRepository) Save(ctx context.Context, appointment *domain.Appointment) error {
	query := `
		INSERT INTO appointments (document_number, date, time, result, type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (document_number, date, type) DO UPDATE SET
			time = EXCLUDED.time,
			result = EXCLUDED.result
		RETURNING id
	`

	var appointmentTime *string
	if appointment.Time != nil {
		t := appointment.Time.Format("15:04:05")
		appointmentTime = &t
	}

	err := r.db.Pool.QueryRow(ctx, query,
		appointment.DocumentNumber.String(),
		appointment.Date,
		appointmentTime,
		appointment.Result,
		appointment.Type.String(),
		appointment.CreatedAt,
	).Scan(&appointment.ID)

	return err
}

// Delete removes an appointment
func (r *AppointmentRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM appointments WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// scanAppointment scans a single appointment from the database
func (r *AppointmentRepository) scanAppointment(ctx context.Context, query string, args ...interface{}) (*domain.Appointment, error) {
	var (
		id              int
		docNumStr       string
		date            time.Time
		timeStr         *string
		result          *string
		appointmentType string
		createdAt       time.Time
	)

	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(
		&id,
		&docNumStr,
		&date,
		&timeStr,
		&result,
		&appointmentType,
		&createdAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	docNum, err := domain.ParseDocumentNumber(docNumStr)
	if err != nil {
		return nil, err
	}

	appointment := &domain.Appointment{
		ID:             id,
		DocumentNumber: docNum,
		Date:           date,
		Result:         result,
		Type:           domain.AppointmentType(appointmentType),
		CreatedAt:      createdAt,
	}

	if timeStr != nil {
		t, err := time.Parse("15:04:05", *timeStr)
		if err == nil {
			appointment.Time = &t
		}
	}

	return appointment, nil
}

// scanAppointments scans multiple appointments from the database
func (r *AppointmentRepository) scanAppointments(ctx context.Context, query string, args ...interface{}) ([]*domain.Appointment, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var appointments []*domain.Appointment
	for rows.Next() {
		var (
			id              int
			docNumStr       string
			date            time.Time
			timeStr         *string
			result          *string
			appointmentType string
			createdAt       time.Time
		)

		if err := rows.Scan(&id, &docNumStr, &date, &timeStr, &result, &appointmentType, &createdAt); err != nil {
			return nil, err
		}

		docNum, err := domain.ParseDocumentNumber(docNumStr)
		if err != nil {
			continue
		}

		appointment := &domain.Appointment{
			ID:             id,
			DocumentNumber: docNum,
			Date:           date,
			Result:         result,
			Type:           domain.AppointmentType(appointmentType),
			CreatedAt:      createdAt,
		}

		if timeStr != nil {
			t, err := time.Parse("15:04:05", *timeStr)
			if err == nil {
				appointment.Time = &t
			}
		}

		appointments = append(appointments, appointment)
	}

	return appointments, rows.Err()
}
