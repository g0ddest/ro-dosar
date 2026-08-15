package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

func timelineDoc(t *testing.T, numStr string, registered time.Time, cat string, solution string, term string) *domain.Document {
	t.Helper()
	docNum, err := domain.ParseDocumentNumber(numStr)
	if err != nil {
		t.Fatal(err)
	}
	doc := domain.NewDocument(docNum, registered, domain.Category(cat))
	if solution != "" {
		sol, err := domain.ParseDocumentNumber(solution)
		if err != nil {
			t.Fatal(err)
		}
		doc.SetSolutionNumber(sol)
	}
	if term != "" {
		tm, err := time.Parse("2006-01-02", term)
		if err != nil {
			t.Fatal(err)
		}
		doc.SetTerm(tm)
	}
	return doc
}

func sp(s string) *string { return &s }

func TestBuildTimeline_FullHistory(t *testing.T) {
	doc := timelineDoc(t, "9352/RD/2022", time.Date(2022, 4, 5, 0, 0, 0, 0, time.UTC), "ART_11", "2265/P/2025", "2025-03-10")
	events := []repository.ChangeEvent{
		{NoticedAt: time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC), NewTerm: sp("2024-06-01")},
		{NoticedAt: time.Date(2024, 5, 2, 12, 0, 0, 0, time.UTC), OldTerm: sp("2024-06-01"), NewTerm: sp("2025-03-10")},
		{NoticedAt: time.Date(2025, 6, 20, 12, 0, 0, 0, time.UTC), OldTerm: sp("2025-03-10"), NewTerm: sp("2025-03-10"), NewSolution: sp("2265/P/2025")},
	}
	ordin := &repository.Ordin{URL: "https://cetatenie.just.ro/x.pdf", Number: 2265, Letter: "P", Date: time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC)}

	tl := buildTimeline(doc, events, func(number int, letter string, year int) *repository.Ordin {
		if number == 2265 && letter == "P" && year == 2025 {
			return ordin
		}
		return nil
	})

	types := []string{}
	for _, e := range tl {
		types = append(types, e.Type)
	}
	want := []string{"REGISTERED", "TERM_SET", "TERM_CHANGED", "SOLUTION_PUBLISHED"}
	if len(tl) != 4 {
		t.Fatalf("expected %v, got %v", want, types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, types)
		}
	}

	if *tl[0].Date != "2022-04-05" {
		t.Errorf("wrong registered date: %+v", tl[0])
	}
	if *tl[1].Term != "2024-06-01" || tl[1].PrevTerm != nil || *tl[1].NoticedAt != "2024-01-10" {
		t.Errorf("wrong TERM_SET: %+v", tl[1])
	}
	if *tl[2].Term != "2025-03-10" || *tl[2].PrevTerm != "2024-06-01" {
		t.Errorf("wrong TERM_CHANGED: %+v", tl[2])
	}
	sol := tl[3]
	if *sol.SolutionNumber != "2265/P/2025" || *sol.Date != "2025-06-18" ||
		*sol.OrdinURL != "https://cetatenie.just.ro/x.pdf" || *sol.NoticedAt != "2025-06-20" {
		t.Errorf("wrong SOLUTION_PUBLISHED: %+v", sol)
	}
}

func TestBuildTimeline_PreAuditDocument(t *testing.T) {
	// term and solution exist but no audit events: synthesized facts, no dates
	doc := timelineDoc(t, "101/A/2021", time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", "15/P/2024", "2024-01-01")

	tl := buildTimeline(doc, nil, func(int, string, int) *repository.Ordin { return nil })

	if len(tl) != 3 || tl[1].Type != "TERM_SET" || tl[2].Type != "SOLUTION_PUBLISHED" {
		t.Fatalf("expected synthesized TERM_SET+SOLUTION, got %+v", tl)
	}
	if tl[1].NoticedAt != nil || tl[1].Date != nil || *tl[1].Term != "2024-01-01" {
		t.Errorf("synthesized TERM_SET must carry only the term: %+v", tl[1])
	}
	if tl[2].NoticedAt != nil || tl[2].Date != nil || *tl[2].SolutionNumber != "15/P/2024" || tl[2].OrdinURL != nil {
		t.Errorf("synthesized SOLUTION without ordin: %+v", tl[2])
	}
}

func TestBuildTimeline_UnsolvedMinimal(t *testing.T) {
	doc := timelineDoc(t, "303/A/2023", time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", "", "")

	tl := buildTimeline(doc, nil, func(int, string, int) *repository.Ordin { return nil })

	if len(tl) != 1 || tl[0].Type != "REGISTERED" {
		t.Fatalf("expected only REGISTERED, got %+v", tl)
	}
}

func TestBuildTimeline_NoDuplicateTermEvent(t *testing.T) {
	// an audit event already covers the current term: no synthesized duplicate
	doc := timelineDoc(t, "1/A/2023", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), "ART_10", "", "2025-05-05")
	events := []repository.ChangeEvent{
		{NoticedAt: time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC), NewTerm: sp("2025-05-05")},
	}

	tl := buildTimeline(doc, events, func(int, string, int) *repository.Ordin { return nil })

	count := 0
	for _, e := range tl {
		if e.Type == "TERM_SET" || e.Type == "TERM_CHANGED" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one term event, got %+v", tl)
	}
}

func TestGetDocument_TimelinePresentAndDegrades(t *testing.T) {
	// full path: solved doc + audit error -> synthesized timeline, still 200
	docRepo := newQueueDocument(t, "101/A/2021", time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", true)
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), &MockStatsRepository{},
		&MockAuditRepository{eventsErr: context.DeadlineExceeded}, &MockOrdinRepository{}, &MockOathRepository{})

	rec := doDocumentRequest(t, handler, "101", "A", "2021")

	if rec.Code != 200 {
		t.Fatalf("audit failure must not fail the request: %d", rec.Code)
	}
	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Timeline) < 2 || resp.Timeline[0].Type != "REGISTERED" {
		t.Errorf("expected synthesized timeline, got %+v", resp.Timeline)
	}
}
