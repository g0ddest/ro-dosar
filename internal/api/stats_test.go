package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// MockStatsRepository is a mock implementation of StatsRepository
type MockStatsRepository struct {
	yearly        []repository.CategoryYearStats
	activity      []repository.CategoryYearActivity
	yearlyErr     error
	activityErr   error
	yearlyCalls   int
	activityCalls int
	cohorts       []repository.CohortCell
	cohortsErr    error
	cohortCalls   int
}

func (r *MockStatsRepository) GetYearlyStats(ctx context.Context) ([]repository.CategoryYearStats, error) {
	r.yearlyCalls++
	return r.yearly, r.yearlyErr
}

func (r *MockStatsRepository) GetRecentActivity(ctx context.Context, since time.Time) ([]repository.CategoryYearActivity, error) {
	r.activityCalls++
	return r.activity, r.activityErr
}

func (r *MockStatsRepository) GetCohortMatrix(ctx context.Context) ([]repository.CohortCell, error) {
	r.cohortCalls++
	return r.cohorts, r.cohortsErr
}

func newStatsHandler(statsRepo repository.StatsRepository) *Handler {
	return NewHandler(NewMockDocumentRepository(), NewMockAppointmentRepository(), statsRepo)
}

func doStatsRequest(t *testing.T, handler *Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	handler.GetStats(rec, req)
	return rec
}

func TestGetStats_ResponseShape(t *testing.T) {
	statsRepo := &MockStatsRepository{
		// deliberately unordered input: ART_10 before ART_8, years descending
		yearly: []repository.CategoryYearStats{
			{Category: "ART_10", Year: 2022, Total: 9120, Solved: 2827, WithTerm: 5000},
			{Category: "ART_10", Year: 2021, Total: 12872, Solved: 10400, WithTerm: 11000},
			{Category: "ART_8", Year: 2023, Total: 500, Solved: 10, WithTerm: 100},
			{Category: "UNKNOWN_CAT", Year: 2021, Total: 5, Solved: 1, WithTerm: 0},
		},
		activity: []repository.CategoryYearActivity{
			{Category: "ART_10", Year: 2022, Solved: 1250, TermsSet: 340},
			{Category: "ART_10", Year: 2020, Solved: 0, TermsSet: 0}, // zero/zero: dropped
			{Category: "ART_10", Year: 2021, Solved: 620, TermsSet: 0},
		},
	}
	handler := newStatsHandler(statsRepo)

	rec := doStatsRequest(t, handler)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp StatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.ActivityWindowDays != 90 {
		t.Errorf("expected activityWindowDays 90, got %d", resp.ActivityWindowDays)
	}
	if resp.GeneratedAt == "" {
		t.Error("expected non-empty generatedAt")
	}

	// unknown categories are skipped; fixed domain order: ART_8 before ART_10
	if len(resp.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(resp.Categories))
	}
	if resp.Categories[0].Category.Code != "ART_8" || resp.Categories[1].Category.Code != "ART_10" {
		t.Errorf("wrong category order: %s, %s",
			resp.Categories[0].Category.Code, resp.Categories[1].Category.Code)
	}

	art10 := resp.Categories[1]
	if art10.Category.Name != "Article 10" || art10.Category.NameRO != "Articolul 10" {
		t.Errorf("wrong category names: %+v", art10.Category)
	}

	// years ascending
	if len(art10.Years) != 2 || art10.Years[0].Year != 2021 || art10.Years[1].Year != 2022 {
		t.Errorf("expected years [2021 2022], got %+v", art10.Years)
	}
	if art10.Years[0].Total != 12872 || art10.Years[0].Solved != 10400 || art10.Years[0].WithTerm != 11000 {
		t.Errorf("wrong 2021 counts: %+v", art10.Years[0])
	}

	// activity: zero/zero dropped, ascending by year
	if len(art10.RecentActivity) != 2 ||
		art10.RecentActivity[0].Year != 2021 || art10.RecentActivity[1].Year != 2022 {
		t.Errorf("expected activity years [2021 2022], got %+v", art10.RecentActivity)
	}
	if art10.RecentActivity[1].Solved != 1250 || art10.RecentActivity[1].TermsSet != 340 {
		t.Errorf("wrong 2022 activity: %+v", art10.RecentActivity[1])
	}

	// ART_8 has no activity rows: must serialize as [] not null
	if !strings.Contains(rec.Body.String(), `"recentActivity":[]`) {
		t.Error("expected empty recentActivity to serialize as []")
	}

	// slim category struct: no legal-text description in the payload
	if strings.Contains(rec.Body.String(), "description") {
		t.Error("stats payload must not contain category descriptions")
	}
}

func TestGetStats_Cache(t *testing.T) {
	statsRepo := &MockStatsRepository{
		yearly: []repository.CategoryYearStats{
			{Category: "ART_10", Year: 2022, Total: 10, Solved: 5, WithTerm: 3},
		},
	}
	handler := newStatsHandler(statsRepo)

	first := doStatsRequest(t, handler)
	second := doStatsRequest(t, handler)

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("expected 200s, got %d and %d", first.Code, second.Code)
	}
	if statsRepo.yearlyCalls != 1 || statsRepo.activityCalls != 1 {
		t.Errorf("expected repository hit once, got yearly=%d activity=%d",
			statsRepo.yearlyCalls, statsRepo.activityCalls)
	}
	if first.Body.String() != second.Body.String() {
		t.Error("cached response differs from first response")
	}
}

func TestGetStats_RepositoryError(t *testing.T) {
	statsRepo := &MockStatsRepository{yearlyErr: context.DeadlineExceeded}
	handler := newStatsHandler(statsRepo)

	rec := doStatsRequest(t, handler)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("invalid error JSON: %v", err)
	}
	if errResp.Status != http.StatusInternalServerError || errResp.Title == "" {
		t.Errorf("malformed RFC 7807 error: %+v", errResp)
	}
}

func doDocumentRequest(t *testing.T, handler *Handler, number, category, year string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/documents/"+number+"/"+category+"/"+year, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("number", number)
	rctx.URLParams.Add("category", category)
	rctx.URLParams.Add("year", year)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler.GetDocument(rec, req)
	return rec
}

func newQueueDocument(t *testing.T, numStr string, registered time.Time, cat string, solved bool) *MockDocumentRepository {
	t.Helper()
	docNum, err := domain.ParseDocumentNumber(numStr)
	if err != nil {
		t.Fatalf("bad document number: %v", err)
	}
	doc := domain.NewDocument(docNum, registered, domain.Category(cat))
	if solved {
		sol, _ := domain.ParseDocumentNumber("1/P/2026")
		doc.SetSolutionNumber(sol)
	}
	repo := NewMockDocumentRepository()
	_ = repo.Save(context.Background(), doc)
	return repo
}

func doCohortsRequest(t *testing.T, handler *Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/stats/cohorts", nil)
	rec := httptest.NewRecorder()
	handler.GetCohortStats(rec, req)
	return rec
}

func TestGetCohortStats_ShapeAndOrder(t *testing.T) {
	statsRepo := &MockStatsRepository{
		cohorts: []repository.CohortCell{
			{Category: "ART_10", RegYear: 2023, SolYear: 2026, Count: 141, P50: 1500, P90: 2900},
			{Category: "ART_8", RegYear: 2023, SolYear: 2025, Count: 55, P50: 300, P90: 580},
			{Category: "UNKNOWN", RegYear: 2020, SolYear: 2021, Count: 5, P50: 1, P90: 2},
		},
	}
	handler := newStatsHandler(statsRepo)

	rec := doCohortsRequest(t, handler)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp CohortStatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.GeneratedAt == "" {
		t.Error("expected generatedAt")
	}
	if len(resp.Categories) != 2 ||
		resp.Categories[0].Category.Code != "ART_8" || resp.Categories[1].Category.Code != "ART_10" {
		t.Fatalf("wrong categories/order: %+v", resp.Categories)
	}
	cell := resp.Categories[1].Cohorts[0]
	if cell.RegYear != 2023 || cell.SolYear != 2026 || cell.Count != 141 || cell.P50 != 1500 || cell.P90 != 2900 {
		t.Errorf("wrong cell: %+v", cell)
	}
	if strings.Contains(rec.Body.String(), "description") {
		t.Error("cohorts payload must not contain category descriptions")
	}
}

func TestGetCohortStats_Cache(t *testing.T) {
	statsRepo := &MockStatsRepository{
		cohorts: []repository.CohortCell{{Category: "ART_10", RegYear: 2023, SolYear: 2026, Count: 1, P50: 1, P90: 1}},
	}
	handler := newStatsHandler(statsRepo)

	doCohortsRequest(t, handler)
	doCohortsRequest(t, handler)

	if statsRepo.cohortCalls != 1 {
		t.Errorf("expected repository hit once, got %d", statsRepo.cohortCalls)
	}
}

func TestGetCohortStats_RepositoryError(t *testing.T) {
	statsRepo := &MockStatsRepository{cohortsErr: context.DeadlineExceeded}
	handler := newStatsHandler(statsRepo)

	rec := doCohortsRequest(t, handler)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetDocument_WaveQueueForUnsolved(t *testing.T) {
	docRepo := newQueueDocument(t, "39946/RD/2024", time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), "ART_11", false)
	fix := art11Fixture()
	statsRepo := &MockStatsRepository{}
	for _, y := range fix.Years {
		statsRepo.yearly = append(statsRepo.yearly, repository.CategoryYearStats{
			Category: "ART_11", Year: y.Year, Total: y.Total, Solved: y.Solved,
		})
	}
	for _, a := range fix.RecentActivity {
		statsRepo.activity = append(statsRepo.activity, repository.CategoryYearActivity{
			Category: "ART_11", Year: a.Year, Solved: a.Solved,
		})
	}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "39946", "RD", "2024")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Queue == nil || resp.Queue.CohortTotal != 45764 || resp.Queue.Percentile != 0.87 {
		t.Fatalf("wrong queue: %+v", resp.Queue)
	}
	if resp.Queue.EstimatedMonthsMin == nil || *resp.Queue.EstimatedMonthsMin != 21 || *resp.Queue.EstimatedMonthsMax != 36 {
		t.Errorf("expected [21, 36], got %+v", resp.Queue)
	}
	for _, gone := range []string{`"ahead"`, `"solvedLast90Days"`, `"solvedLastYear"`, `"estimatedMonths"`} {
		if gone == `"estimatedMonths"` {
			continue // substring of estimatedMonthsMin/Max
		}
		if strings.Contains(rec.Body.String(), gone) {
			t.Errorf("legacy field %s must be gone", gone)
		}
	}
}

func TestGetDocument_NoQueueForSolved(t *testing.T) {
	docRepo := newQueueDocument(t, "101/A/2021", time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", true)
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), &MockStatsRepository{})

	rec := doDocumentRequest(t, handler, "101", "A", "2021")

	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"queue"`) {
		t.Errorf("solved document must not carry queue: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGetDocument_QueueAbsentWhenCategoryUnknown(t *testing.T) {
	docRepo := newQueueDocument(t, "10/A/2024", time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{} // no yearly stats at all
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "10", "A", "2024")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"queue"`) {
		t.Error("queue must be absent when the category has no stats")
	}
}

func TestGetDocument_QueueErrorNonFatal(t *testing.T) {
	docRepo := newQueueDocument(t, "10/A/2024", time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{yearlyErr: context.DeadlineExceeded}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "10", "A", "2024")

	if rec.Code != http.StatusOK {
		t.Fatalf("stats error must not fail the document request, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"queue"`) {
		t.Error("queue must be absent when stats are unavailable")
	}
}
