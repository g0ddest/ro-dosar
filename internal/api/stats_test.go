package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
}

func (r *MockStatsRepository) GetYearlyStats(ctx context.Context) ([]repository.CategoryYearStats, error) {
	r.yearlyCalls++
	return r.yearly, r.yearlyErr
}

func (r *MockStatsRepository) GetRecentActivity(ctx context.Context, since time.Time) ([]repository.CategoryYearActivity, error) {
	r.activityCalls++
	return r.activity, r.activityErr
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
