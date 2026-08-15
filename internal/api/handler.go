package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// Handler handles API requests
type Handler struct {
	documentRepo    repository.DocumentRepository
	appointmentRepo repository.AppointmentRepository
	statsRepo       repository.StatsRepository
	auditRepo       repository.DocumentAuditRepository
	ordinRepo       repository.OrdinRepository
	oathRepo        repository.OathRepository

	statsMu      sync.Mutex
	statsCache   *StatsResponse
	statsCacheAt time.Time

	cohortMu      sync.Mutex
	cohortCache   *CohortStatsResponse
	cohortCacheAt time.Time
}

// NewHandler creates a new API handler
func NewHandler(documentRepo repository.DocumentRepository, appointmentRepo repository.AppointmentRepository, statsRepo repository.StatsRepository, auditRepo repository.DocumentAuditRepository, ordinRepo repository.OrdinRepository, oathRepo repository.OathRepository) *Handler {
	return &Handler{
		documentRepo:    documentRepo,
		appointmentRepo: appointmentRepo,
		statsRepo:       statsRepo,
		auditRepo:       auditRepo,
		ordinRepo:       ordinRepo,
		oathRepo:        oathRepo,
	}
}

// CategoryResponse represents a category in API responses
type CategoryResponse struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	NameRO      string `json:"nameRO"`
	Description string `json:"description"`
}

// DocumentResponse represents a document in API responses
type DocumentResponse struct {
	DocumentNumber string                  `json:"documentNumber"`
	RegisteredAt   string                  `json:"registeredAt"`
	Category       CategoryResponse        `json:"category"`
	Term           *string                 `json:"term,omitempty"`
	SolutionNumber *string                 `json:"solutionNumber,omitempty"`
	Appointments   []AppointmentResponse   `json:"appointments,omitempty"`
	Queue          *QueueResponse          `json:"queue,omitempty"`
	Timeline       []TimelineEventResponse `json:"timeline"`
	Oath           *OathResponse           `json:"oath,omitempty"`
}

// OathResponse is a scheduled oath ceremony for a solved dossier
type OathResponse struct {
	Date    string  `json:"date"`
	Time    *string `json:"time,omitempty"`
	ListURL string  `json:"listUrl"`
}

// AppointmentResponse represents an appointment in API responses
type AppointmentResponse struct {
	Date   string  `json:"date"`
	Time   *string `json:"time,omitempty"`
	Result *string `json:"result,omitempty"`
	Type   string  `json:"type"`
}

// ErrorResponse represents an error in API responses (RFC 7807)
type ErrorResponse struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

// documentToResponse converts a domain document to API response
func (h *Handler) documentToResponse(doc *domain.Document, appointments []*domain.Appointment) DocumentResponse {
	categoryInfo := doc.Category.Info()
	response := DocumentResponse{
		DocumentNumber: doc.DocumentNumber.String(),
		RegisteredAt:   doc.RegisteredAt.Format("2006-01-02"),
		Category: CategoryResponse{
			Code:        categoryInfo.Code,
			Name:        categoryInfo.Name,
			NameRO:      categoryInfo.NameRO,
			Description: categoryInfo.Description,
		},
	}

	if doc.Term != nil {
		term := doc.Term.Format("2006-01-02")
		response.Term = &term
	}

	if doc.SolutionNumber != nil {
		solNum := doc.SolutionNumber.String()
		response.SolutionNumber = &solNum
	}

	for _, apt := range appointments {
		aptResponse := AppointmentResponse{
			Date: apt.Date.Format("2006-01-02"),
			Type: string(apt.Type),
		}

		if apt.Time != nil {
			time := apt.Time.Format("15:04")
			aptResponse.Time = &time
		}

		if apt.Result != nil {
			aptResponse.Result = apt.Result
		}

		response.Appointments = append(response.Appointments, aptResponse)
	}

	return response
}

// GetDocument handles GET /api/v1/documents/{number}/{category}/{year}
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	number := chi.URLParam(r, "number")
	category := chi.URLParam(r, "category")
	year := chi.URLParam(r, "year")

	docNumStr := number + "/" + category + "/" + year
	docNum, err := domain.ParseDocumentNumber(docNumStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid document number", err.Error())
		return
	}

	doc, err := h.documentRepo.GetByNumber(r.Context(), docNum)
	if err != nil {
		if errors.Is(err, domain.ErrDocumentNotFound) {
			h.writeError(w, http.StatusNotFound, "Not Found", "Document "+docNumStr+" not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	// Get appointments for this document
	appointments, err := h.appointmentRepo.GetByDocumentNumber(r.Context(), docNum)
	if err != nil && !errors.Is(err, domain.ErrAppointmentNotFound) {
		h.writeError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	response := h.documentToResponse(doc, appointments)
	if doc.SolutionNumber == nil {
		if queue, err := h.buildQueueInfo(r.Context(), doc); err == nil && queue != nil {
			response.Queue = queue
		}
	}
	response.Timeline = h.buildDocumentTimeline(r.Context(), doc)
	if doc.SolutionNumber != nil {
		if oath, err := h.oathRepo.GetByDossier(r.Context(), doc.DocumentNumber.Number, doc.DocumentNumber.Year); err == nil && oath != nil {
			response.Oath = &OathResponse{
				Date:    oath.Date.Format("2006-01-02"),
				Time:    oath.Time,
				ListURL: oath.ListURL,
			}
		}
	}
	h.writeJSON(w, http.StatusOK, response)
}

// writeJSON writes a JSON response
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response in RFC 7807 format
func (h *Handler) writeError(w http.ResponseWriter, status int, title, detail string) {
	errType := "https://tools.ietf.org/html/rfc7231#section-6.5.4"
	switch status {
	case http.StatusBadRequest:
		errType = "https://tools.ietf.org/html/rfc7231#section-6.5.1"
	case http.StatusInternalServerError:
		errType = "https://tools.ietf.org/html/rfc7231#section-6.6.1"
	}

	h.writeJSON(w, status, ErrorResponse{
		Type:   errType,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}
