package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

const (
	activityWindowDays = 90
	statsCacheTTL      = 15 * time.Minute
	statsErrorCacheTTL = 30 * time.Second
)

// statsCategoryOrder is the fixed presentation order for categories
var statsCategoryOrder = []domain.Category{
	domain.CategoryArt8,
	domain.CategoryArt8_1,
	domain.CategoryArt8_2,
	domain.CategoryArt10,
	domain.CategoryArt11,
}

// StatsCategoryRef is a slim category reference for stats responses.
// CategoryResponse is not reused: its description carries multi-paragraph
// legal text that would bloat every stats payload.
type StatsCategoryRef struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	NameRO string `json:"nameRO"`
}

// YearStatsResponse is per-registration-year progress within a category
type YearStatsResponse struct {
	Year     int `json:"year"`
	Total    int `json:"total"`
	Solved   int `json:"solved"`
	WithTerm int `json:"withTerm"`
}

// YearActivityResponse is recent activity for one registration year
type YearActivityResponse struct {
	Year     int `json:"year"`
	Solved   int `json:"solved"`
	TermsSet int `json:"termsSet"`
}

// CategoryStatsResponse groups stats for one category
type CategoryStatsResponse struct {
	Category       StatsCategoryRef       `json:"category"`
	Years          []YearStatsResponse    `json:"years"`
	RecentActivity []YearActivityResponse `json:"recentActivity"`
}

// StatsResponse is the GET /api/v1/stats payload
type StatsResponse struct {
	GeneratedAt        string                  `json:"generatedAt"`
	ActivityWindowDays int                     `json:"activityWindowDays"`
	Categories         []CategoryStatsResponse `json:"categories"`
}

// QueueResponse is the wave-model estimate for an unsolved document
type QueueResponse struct {
	CohortTotal        int     `json:"cohortTotal"`
	Percentile         float64 `json:"percentile"`
	WavePassed         bool    `json:"wavePassed"`
	EstimatedMonthsMin *int    `json:"estimatedMonthsMin,omitempty"`
	EstimatedMonthsMax *int    `json:"estimatedMonthsMax,omitempty"`
}

// buildStatsResponse assembles the stats payload: fixed category order,
// years and activity ascending, zero/zero activity rows dropped,
// unknown categories skipped
func buildStatsResponse(yearly []repository.CategoryYearStats, activity []repository.CategoryYearActivity, now time.Time) *StatsResponse {
	yearsByCategory := make(map[string][]YearStatsResponse)
	for _, s := range yearly {
		yearsByCategory[s.Category] = append(yearsByCategory[s.Category], YearStatsResponse{
			Year:     s.Year,
			Total:    s.Total,
			Solved:   s.Solved,
			WithTerm: s.WithTerm,
		})
	}

	activityByCategory := make(map[string][]YearActivityResponse)
	for _, a := range activity {
		if a.Solved == 0 && a.TermsSet == 0 {
			continue
		}
		activityByCategory[a.Category] = append(activityByCategory[a.Category], YearActivityResponse{
			Year:     a.Year,
			Solved:   a.Solved,
			TermsSet: a.TermsSet,
		})
	}

	resp := &StatsResponse{
		GeneratedAt:        now.UTC().Format(time.RFC3339),
		ActivityWindowDays: activityWindowDays,
		Categories:         []CategoryStatsResponse{},
	}

	for _, cat := range statsCategoryOrder {
		years, ok := yearsByCategory[cat.String()]
		if !ok {
			continue
		}
		sort.Slice(years, func(i, j int) bool { return years[i].Year < years[j].Year })

		catActivity := activityByCategory[cat.String()]
		if catActivity == nil {
			catActivity = []YearActivityResponse{}
		}
		sort.Slice(catActivity, func(i, j int) bool { return catActivity[i].Year < catActivity[j].Year })

		info := cat.Info()
		resp.Categories = append(resp.Categories, CategoryStatsResponse{
			Category: StatsCategoryRef{
				Code:   info.Code,
				Name:   info.Name,
				NameRO: info.NameRO,
			},
			Years:          years,
			RecentActivity: catActivity,
		})
	}

	return resp
}

// getStatsCached returns the cached stats response, recomputing it when stale
func (h *Handler) getStatsCached(ctx context.Context) (*StatsResponse, error) {
	return h.statsCache.get(ctx, statsCacheTTL, statsErrorCacheTTL, h.loadStats)
}

// loadStats queries the stats repository and assembles the stats payload
func (h *Handler) loadStats(ctx context.Context) (*StatsResponse, error) {
	yearly, err := h.statsRepo.GetYearlyStats(ctx)
	if err != nil {
		return nil, err
	}

	since := time.Now().AddDate(0, 0, -activityWindowDays)
	activity, err := h.statsRepo.GetRecentActivity(ctx, since)
	if err != nil {
		return nil, err
	}

	return buildStatsResponse(yearly, activity, time.Now()), nil
}

// GetStats handles GET /api/v1/stats
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.getStatsCached(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// buildQueueInfo computes the wave-model estimate for an unsolved document
// from the cached stats payloads; no per-request SQL is involved
func (h *Handler) buildQueueInfo(ctx context.Context, doc *domain.Document) (*QueueResponse, error) {
	stats, err := h.getStatsCached(ctx)
	if err != nil {
		return nil, err
	}

	var cells []CohortCellResponse
	if cohorts, err := h.getCohortStatsCached(ctx); err == nil {
		for _, cc := range cohorts.Categories {
			if cc.Category.Code == doc.Category.String() {
				cells = cc.Cohorts
				break
			}
		}
	}

	for _, cat := range stats.Categories {
		if cat.Category.Code == doc.Category.String() {
			return buildWaveEstimate(cat, cells, doc.RegisteredAt.Year(), doc.DocumentNumber.Number, time.Now()), nil
		}
	}
	return nil, nil
}

// CohortCellResponse is one (registration year, solution year) cell
type CohortCellResponse struct {
	RegYear int `json:"regYear"`
	SolYear int `json:"solYear"`
	Count   int `json:"count"`
	P50     int `json:"p50"`
	P90     int `json:"p90"`
}

// CategoryCohortsResponse groups cohort cells for one category
type CategoryCohortsResponse struct {
	Category StatsCategoryRef     `json:"category"`
	Cohorts  []CohortCellResponse `json:"cohorts"`
}

// CohortStatsResponse is the GET /api/v1/stats/cohorts payload
type CohortStatsResponse struct {
	GeneratedAt string                    `json:"generatedAt"`
	Categories  []CategoryCohortsResponse `json:"categories"`
}

// buildCohortResponse assembles the matrix payload in fixed category order
func buildCohortResponse(cells []repository.CohortCell, now time.Time) *CohortStatsResponse {
	byCategory := make(map[string][]CohortCellResponse)
	for _, c := range cells {
		byCategory[c.Category] = append(byCategory[c.Category], CohortCellResponse{
			RegYear: c.RegYear, SolYear: c.SolYear, Count: c.Count, P50: c.P50, P90: c.P90,
		})
	}

	resp := &CohortStatsResponse{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Categories:  []CategoryCohortsResponse{},
	}
	for _, cat := range statsCategoryOrder {
		cohorts, ok := byCategory[cat.String()]
		if !ok {
			continue
		}
		sort.Slice(cohorts, func(i, j int) bool {
			if cohorts[i].RegYear != cohorts[j].RegYear {
				return cohorts[i].RegYear < cohorts[j].RegYear
			}
			return cohorts[i].SolYear < cohorts[j].SolYear
		})
		info := cat.Info()
		resp.Categories = append(resp.Categories, CategoryCohortsResponse{
			Category: StatsCategoryRef{Code: info.Code, Name: info.Name, NameRO: info.NameRO},
			Cohorts:  cohorts,
		})
	}
	return resp
}

// getCohortStatsCached returns the cached cohort matrix, recomputing it when stale
func (h *Handler) getCohortStatsCached(ctx context.Context) (*CohortStatsResponse, error) {
	return h.cohortCache.get(ctx, statsCacheTTL, statsErrorCacheTTL, h.loadCohortStats)
}

// loadCohortStats queries the cohort matrix and assembles the cohorts payload
func (h *Handler) loadCohortStats(ctx context.Context) (*CohortStatsResponse, error) {
	cells, err := h.statsRepo.GetCohortMatrix(ctx)
	if err != nil {
		return nil, err
	}
	return buildCohortResponse(cells, time.Now()), nil
}

// GetCohortStats handles GET /api/v1/stats/cohorts
func (h *Handler) GetCohortStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.getCohortStatsCached(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}
