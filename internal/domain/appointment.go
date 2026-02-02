package domain

import (
	"time"
)

// AppointmentType represents the type of appointment
type AppointmentType string

const (
	AppointmentTypeInvitation AppointmentType = "INVITATION"
	AppointmentTypeResult     AppointmentType = "RESULT"
)

// IsValid checks if the appointment type is valid
func (t AppointmentType) IsValid() bool {
	return t == AppointmentTypeInvitation || t == AppointmentTypeResult
}

// String returns the string representation
func (t AppointmentType) String() string {
	return string(t)
}

// Appointment represents an appointment (invitation or result) for a document
type Appointment struct {
	ID             int
	DocumentNumber DocumentNumber
	Date           time.Time
	Time           *time.Time      // for invitation
	Result         *string         // for result: Aviz pozitiv, Absent, Amânare
	Type           AppointmentType // INVITATION, RESULT
	CreatedAt      time.Time
}

// NewAppointment creates a new Appointment entity
func NewAppointment(docNum DocumentNumber, date time.Time, appointmentType AppointmentType) *Appointment {
	return &Appointment{
		DocumentNumber: docNum,
		Date:           date,
		Type:           appointmentType,
		CreatedAt:      time.Now(),
	}
}

// NewInvitationAppointment creates an invitation appointment with time
func NewInvitationAppointment(docNum DocumentNumber, date time.Time, appointmentTime time.Time) *Appointment {
	return &Appointment{
		DocumentNumber: docNum,
		Date:           date,
		Time:           &appointmentTime,
		Type:           AppointmentTypeInvitation,
		CreatedAt:      time.Now(),
	}
}

// NewResultAppointment creates a result appointment with result string
func NewResultAppointment(docNum DocumentNumber, date time.Time, result string) *Appointment {
	return &Appointment{
		DocumentNumber: docNum,
		Date:           date,
		Result:         &result,
		Type:           AppointmentTypeResult,
		CreatedAt:      time.Now(),
	}
}

// SetTime sets the appointment time
func (a *Appointment) SetTime(t time.Time) {
	a.Time = &t
}

// SetResult sets the appointment result
func (a *Appointment) SetResult(result string) {
	a.Result = &result
}

// IsInvitation checks if this is an invitation appointment
func (a *Appointment) IsInvitation() bool {
	return a.Type == AppointmentTypeInvitation
}

// IsResult checks if this is a result appointment
func (a *Appointment) IsResult() bool {
	return a.Type == AppointmentTypeResult
}

// HasChanges checks if the appointment has changes compared to another
func (a *Appointment) HasChanges(other *Appointment) bool {
	if other == nil {
		return true
	}

	if !a.DocumentNumber.Equals(other.DocumentNumber) {
		return true
	}

	if !a.Date.Equal(other.Date) {
		return true
	}

	if a.Type != other.Type {
		return true
	}

	// Compare Time
	if (a.Time == nil) != (other.Time == nil) {
		return true
	}
	if a.Time != nil && other.Time != nil && !a.Time.Equal(*other.Time) {
		return true
	}

	// Compare Result
	if (a.Result == nil) != (other.Result == nil) {
		return true
	}
	if a.Result != nil && other.Result != nil && *a.Result != *other.Result {
		return true
	}

	return false
}
