package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// MockDocumentRepository is a mock implementation of DocumentRepository
type MockDocumentRepository struct {
	documents map[string]*domain.Document
}

func NewMockDocumentRepository() *MockDocumentRepository {
	return &MockDocumentRepository{
		documents: make(map[string]*domain.Document),
	}
}

func (r *MockDocumentRepository) GetByNumber(ctx context.Context, docNum domain.DocumentNumber) (*domain.Document, error) {
	doc, ok := r.documents[docNum.String()]
	if !ok {
		return nil, domain.ErrDocumentNotFound
	}
	return doc, nil
}

func (r *MockDocumentRepository) Save(ctx context.Context, doc *domain.Document) error {
	r.documents[doc.DocumentNumber.String()] = doc
	return nil
}

func (r *MockDocumentRepository) Delete(ctx context.Context, docNum domain.DocumentNumber) error {
	delete(r.documents, docNum.String())
	return nil
}

func (r *MockDocumentRepository) List(ctx context.Context, filter repository.DocumentFilter) ([]*domain.Document, error) {
	var docs []*domain.Document
	for _, doc := range r.documents {
		docs = append(docs, doc)
	}
	return docs, nil
}

// MockAppointmentRepository is a mock implementation of AppointmentRepository
type MockAppointmentRepository struct {
	appointments map[string][]*domain.Appointment
}

func NewMockAppointmentRepository() *MockAppointmentRepository {
	return &MockAppointmentRepository{
		appointments: make(map[string][]*domain.Appointment),
	}
}

func (r *MockAppointmentRepository) GetByID(ctx context.Context, id int) (*domain.Appointment, error) {
	return nil, domain.ErrAppointmentNotFound
}

func (r *MockAppointmentRepository) GetByDocumentNumber(ctx context.Context, docNum domain.DocumentNumber) ([]*domain.Appointment, error) {
	apts, ok := r.appointments[docNum.String()]
	if !ok {
		return nil, nil
	}
	return apts, nil
}

func (r *MockAppointmentRepository) GetByDocumentAndType(ctx context.Context, docNum domain.DocumentNumber, appointmentType domain.AppointmentType) ([]*domain.Appointment, error) {
	return nil, nil
}

func (r *MockAppointmentRepository) Save(ctx context.Context, appointment *domain.Appointment) error {
	key := appointment.DocumentNumber.String()
	r.appointments[key] = append(r.appointments[key], appointment)
	return nil
}

func (r *MockAppointmentRepository) Delete(ctx context.Context, id int) error {
	return nil
}

func TestGetDocument(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	aptRepo := NewMockAppointmentRepository()

	// Create test document
	docNum := domain.DocumentNumber{Number: 10435, Category: "A", Year: 2025}
	registeredAt := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	term := time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC)

	doc := domain.NewDocument(docNum, registeredAt, domain.CategoryArt8)
	doc.SetTerm(term)
	docRepo.Save(context.Background(), doc)

	// Create test appointment
	aptDate := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)
	apt := domain.NewAppointment(docNum, aptDate, domain.AppointmentTypeResult)
	result := "Aviz pozitiv"
	apt.SetResult(result)
	aptRepo.Save(context.Background(), apt)

	handler := NewHandler(docRepo, aptRepo, &MockStatsRepository{},
		&MockAuditRepository{}, &MockOrdinRepository{})
	router := chi.NewRouter()
	router.Get("/documents/{number}/{category}/{year}", handler.GetDocument)

	t.Run("document found with appointments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/documents/10435/A/2025", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("GetDocument() status = %d, want %d", w.Code, http.StatusOK)
		}

		var response DocumentResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.DocumentNumber != "10435/A/2025" {
			t.Errorf("response.DocumentNumber = %s, want 10435/A/2025", response.DocumentNumber)
		}
		if response.RegisteredAt != "2025-01-15" {
			t.Errorf("response.RegisteredAt = %s, want 2025-01-15", response.RegisteredAt)
		}
		if response.Category.Code != "ART_8" {
			t.Errorf("response.Category.Code = %s, want ART_8", response.Category.Code)
		}
		if response.Category.Name != "Article 8" {
			t.Errorf("response.Category.Name = %s, want Article 8", response.Category.Name)
		}
		if response.Term == nil || *response.Term != "2025-07-15" {
			t.Errorf("response.Term = %v, want 2025-07-15", response.Term)
		}
		if len(response.Appointments) != 1 {
			t.Errorf("len(response.Appointments) = %d, want 1", len(response.Appointments))
		}
		if response.Appointments[0].Result == nil || *response.Appointments[0].Result != "Aviz pozitiv" {
			t.Errorf("response.Appointments[0].Result = %v, want 'Aviz pozitiv'", response.Appointments[0].Result)
		}
	})

	t.Run("document not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/documents/99999/A/2025", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("GetDocument() status = %d, want %d", w.Code, http.StatusNotFound)
		}

		var response ErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Status != 404 {
			t.Errorf("response.Status = %d, want 404", response.Status)
		}
		if response.Title != "Not Found" {
			t.Errorf("response.Title = %s, want 'Not Found'", response.Title)
		}
	})

	t.Run("invalid document number", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/documents/abc/A/2025", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("GetDocument() status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}
