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
	yearly          []repository.CategoryYearStats
	activity        []repository.CategoryYearActivity
	yearlyErr       error
	activityErr     error
	yearlyCalls     int
	activityCalls   int
	aheadCount      int
	aheadErr        error
	solvedInYear    int
	solvedInYearErr error
}

func (r *MockStatsRepository) GetYearlyStats(ctx context.Context) ([]repository.CategoryYearStats, error) {
	r.yearlyCalls++
	return r.yearly, r.yearlyErr
}

func (r *MockStatsRepository) GetRecentActivity(ctx context.Context, since time.Time) ([]repository.CategoryYearActivity, error) {
	r.activityCalls++
	return r.activity, r.activityErr
}

func (r *MockStatsRepository) CountAheadInQueue(ctx context.Context, category string, year, number int) (int, error) {
	return r.aheadCount, r.aheadErr
}

func (r *MockStatsRepository) CountSolvedInYear(ctx context.Context, category string, solutionYear int) (int, error) {
	return r.solvedInYear, r.solvedInYearErr
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

func TestGetDocument_QueueForUnsolved(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{
		aheadCount: 4210,
		// yearly rows are required: buildStatsResponse only keeps categories
		// that have yearly stats, and buildQueueInfo reads pace from that response
		yearly: []repository.CategoryYearStats{
			{Category: "ART_10", Year: 2021, Total: 100, Solved: 50, WithTerm: 60},
			{Category: "ART_10", Year: 2022, Total: 100, Solved: 10, WithTerm: 20},
			{Category: "ART_8", Year: 2022, Total: 10, Solved: 5, WithTerm: 5},
		},
		activity: []repository.CategoryYearActivity{
			{Category: "ART_10", Year: 2021, Solved: 50},
			{Category: "ART_10", Year: 2022, Solved: 900},
			{Category: "ART_8", Year: 2022, Solved: 999}, // other category: excluded
		},
	}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Queue == nil {
		t.Fatal("expected queue block for unsolved document")
	}
	if resp.Queue.Ahead != 4210 || resp.Queue.SolvedLast90Days != 950 {
		t.Errorf("wrong queue counts: %+v", resp.Queue)
	}
	// ceil(4210 / (950/3)) = ceil(13.29) = 14
	if resp.Queue.EstimatedMonths == nil || *resp.Queue.EstimatedMonths != 14 {
		t.Errorf("expected estimatedMonths 14, got %+v", resp.Queue.EstimatedMonths)
	}
}

func TestGetDocument_QueuePaceUnknown(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{aheadCount: 77}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Queue == nil || resp.Queue.Ahead != 77 || resp.Queue.SolvedLast90Days != 0 {
		t.Fatalf("wrong queue: %+v", resp.Queue)
	}
	if resp.Queue.EstimatedMonths != nil {
		t.Error("expected estimatedMonths omitted when pace is 0")
	}
	if strings.Contains(rec.Body.String(), "estimatedMonths") {
		t.Error("estimatedMonths must be absent from JSON when pace is 0")
	}
}

func TestGetDocument_NoQueueForSolved(t *testing.T) {
	docRepo := newQueueDocument(t, "101/A/2021", time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", true)
	statsRepo := &MockStatsRepository{aheadCount: 4210}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "101", "A", "2021")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"queue"`) {
		t.Error("solved document must not carry a queue block")
	}
}

func TestGetDocument_QueueErrorNonFatal(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{aheadErr: context.DeadlineExceeded}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	if rec.Code != http.StatusOK {
		t.Fatalf("queue error must not fail the document request, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"queue"`) {
		t.Error("queue must be absent when its computation fails")
	}
}

func TestGetDocument_QueueLastYearFallback(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{
		aheadCount:   4210,
		solvedInYear: 9800,
		// yearly present so the category exists in the stats payload,
		// but no recent activity -> 90-day pace is 0
		yearly: []repository.CategoryYearStats{
			{Category: "ART_10", Year: 2022, Total: 100, Solved: 10, WithTerm: 20},
		},
	}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Queue == nil || resp.Queue.SolvedLast90Days != 0 {
		t.Fatalf("wrong queue: %+v", resp.Queue)
	}
	if resp.Queue.SolvedLastYear == nil || *resp.Queue.SolvedLastYear != 9800 {
		t.Errorf("expected solvedLastYear 9800, got %+v", resp.Queue.SolvedLastYear)
	}
	// ceil(4210 / (9800/12)) = ceil(5.155...) = 6
	if resp.Queue.EstimatedMonths == nil || *resp.Queue.EstimatedMonths != 6 {
		t.Errorf("expected estimatedMonths 6, got %+v", resp.Queue.EstimatedMonths)
	}
}

func TestGetDocument_QueueNoLastYearOnRecentPath(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{
		aheadCount:   100,
		solvedInYear: 9800, // must be ignored: recent pace is available
		yearly: []repository.CategoryYearStats{
			{Category: "ART_10", Year: 2022, Total: 100, Solved: 10, WithTerm: 20},
		},
		activity: []repository.CategoryYearActivity{
			{Category: "ART_10", Year: 2022, Solved: 300},
		},
	}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	if strings.Contains(rec.Body.String(), "solvedLastYear") {
		t.Error("solvedLastYear must be absent when the recent pace is used")
	}
	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// ceil(100 / (300/3)) = 1
	if resp.Queue == nil || resp.Queue.EstimatedMonths == nil || *resp.Queue.EstimatedMonths != 1 {
		t.Errorf("recent-pace estimate broken: %+v", resp.Queue)
	}
}

func TestGetDocument_QueueBothPacesZero(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{aheadCount: 77}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Queue == nil || resp.Queue.Ahead != 77 {
		t.Fatalf("wrong queue: %+v", resp.Queue)
	}
	if resp.Queue.EstimatedMonths != nil || resp.Queue.SolvedLastYear != nil {
		t.Errorf("expected unknown-pace shape, got %+v", resp.Queue)
	}
}

func TestGetDocument_QueueFallbackErrorNonFatal(t *testing.T) {
	docRepo := newQueueDocument(t, "201/A/2022", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", false)
	statsRepo := &MockStatsRepository{aheadCount: 77, solvedInYearErr: context.DeadlineExceeded}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo)

	rec := doDocumentRequest(t, handler, "201", "A", "2022")

	if rec.Code != http.StatusOK {
		t.Fatalf("fallback error must not fail the request, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"queue"`) {
		t.Error("queue must be absent when the fallback lookup fails")
	}
}
