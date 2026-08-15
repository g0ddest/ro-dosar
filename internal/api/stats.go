package api

import (
	"context"
	"math"
	"net/http"
	"sort"
	"time"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

const (
	activityWindowDays = 90
	statsCacheTTL      = 15 * time.Minute
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

// QueueResponse describes an unsolved document's position in its category queue
type QueueResponse struct {
	Ahead            int  `json:"ahead"`
	SolvedLast90Days int  `json:"solvedLast90Days"`
	EstimatedMonths  *int `json:"estimatedMonths,omitempty"`
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
	h.statsMu.Lock()
	if h.statsCache != nil && time.Since(h.statsCacheAt) < statsCacheTTL {
		cached := h.statsCache
		h.statsMu.Unlock()
		return cached, nil
	}
	h.statsMu.Unlock()

	yearly, err := h.statsRepo.GetYearlyStats(ctx)
	if err != nil {
		return nil, err
	}

	since := time.Now().AddDate(0, 0, -activityWindowDays)
	activity, err := h.statsRepo.GetRecentActivity(ctx, since)
	if err != nil {
		return nil, err
	}

	resp := buildStatsResponse(yearly, activity, time.Now())

	h.statsMu.Lock()
	h.statsCache = resp
	h.statsCacheAt = time.Now()
	h.statsMu.Unlock()

	return resp, nil
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

// buildQueueInfo computes an unsolved document's queue position and a rough
// linear estimate from the recent processing pace
func (h *Handler) buildQueueInfo(ctx context.Context, doc *domain.Document) (*QueueResponse, error) {
	ahead, err := h.statsRepo.CountAheadInQueue(ctx, doc.Category.String(), doc.RegisteredAt.Year(), doc.DocumentNumber.Number)
	if err != nil {
		return nil, err
	}

	stats, err := h.getStatsCached(ctx)
	if err != nil {
		return nil, err
	}

	solved90 := 0
	for _, cat := range stats.Categories {
		if cat.Category.Code == doc.Category.String() {
			for _, a := range cat.RecentActivity {
				solved90 += a.Solved
			}
		}
	}

	queue := &QueueResponse{Ahead: ahead, SolvedLast90Days: solved90}
	if solved90 > 0 {
		months := int(math.Ceil(float64(ahead) / (float64(solved90) / 3.0)))
		queue.EstimatedMonths = &months
	}
	return queue, nil
}
