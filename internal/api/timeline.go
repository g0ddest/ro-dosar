package api

import (
	"context"

	"ro-dosar/internal/domain"
	"ro-dosar/internal/repository"
)

// TimelineEventResponse is one dossier-timeline event
type TimelineEventResponse struct {
	Type           string  `json:"type"`
	Date           *string `json:"date,omitempty"`
	NoticedAt      *string `json:"noticedAt,omitempty"`
	Term           *string `json:"term,omitempty"`
	PrevTerm       *string `json:"prevTerm,omitempty"`
	SolutionNumber *string `json:"solutionNumber,omitempty"`
	OrdinURL       *string `json:"ordinUrl,omitempty"`
}

func strPtr(s string) *string { return &s }

// buildTimeline assembles the dossier timeline from the document, its audit
// transitions and an ordin lookup. findOrdin returns nil when no ordin
// matches — lookups are best-effort and never fail the assembly.
func buildTimeline(doc *domain.Document, events []repository.ChangeEvent, findOrdin func(number int, letter string, year int) *repository.Ordin) []TimelineEventResponse {
	timeline := []TimelineEventResponse{{
		Type: "REGISTERED",
		Date: strPtr(doc.RegisteredAt.Format("2006-01-02")),
	}}

	termSeen, solutionSeen := false, false
	var observed []TimelineEventResponse
	for _, ev := range events {
		if ev.NewTerm != nil && (ev.OldTerm == nil || *ev.OldTerm != *ev.NewTerm) {
			e := TimelineEventResponse{
				NoticedAt: strPtr(ev.NoticedAt.Format("2006-01-02")),
				Term:      ev.NewTerm,
			}
			if ev.OldTerm == nil {
				e.Type = "TERM_SET"
			} else {
				e.Type = "TERM_CHANGED"
				e.PrevTerm = ev.OldTerm
			}
			observed = append(observed, e)
			termSeen = true
		}
		if ev.NewSolution != nil && ev.OldSolution == nil {
			e := solutionEvent(*ev.NewSolution, findOrdin)
			e.NoticedAt = strPtr(ev.NoticedAt.Format("2006-01-02"))
			observed = append(observed, e)
			solutionSeen = true
		}
	}

	if !termSeen && doc.Term != nil {
		timeline = append(timeline, TimelineEventResponse{
			Type: "TERM_SET",
			Term: strPtr(doc.Term.Format("2006-01-02")),
		})
	}

	timeline = append(timeline, observed...)

	if !solutionSeen && doc.SolutionNumber != nil {
		timeline = append(timeline, solutionEvent(doc.SolutionNumber.String(), findOrdin))
	}

	return timeline
}

// solutionEvent builds a SOLUTION_PUBLISHED event, enriched with the ordin's
// official date and PDF url when the index knows it
func solutionEvent(solution string, findOrdin func(int, string, int) *repository.Ordin) TimelineEventResponse {
	e := TimelineEventResponse{
		Type:           "SOLUTION_PUBLISHED",
		SolutionNumber: strPtr(solution),
	}
	solNum, err := domain.ParseDocumentNumber(solution)
	if err != nil {
		return e
	}
	if ordin := findOrdin(solNum.Number, solNum.Category, solNum.Year); ordin != nil {
		e.Date = strPtr(ordin.Date.Format("2006-01-02"))
		e.OrdinURL = strPtr(ordin.URL)
	}
	return e
}

// buildDocumentTimeline gathers timeline inputs with best-effort semantics:
// audit or ordin failures degrade to the synthesized-minimum timeline
func (h *Handler) buildDocumentTimeline(ctx context.Context, doc *domain.Document) []TimelineEventResponse {
	events, err := h.auditRepo.GetChangeEvents(ctx, doc.DocumentNumber)
	if err != nil {
		events = nil
	}
	return buildTimeline(doc, events, func(number int, letter string, year int) *repository.Ordin {
		ordin, err := h.ordinRepo.GetBySolution(ctx, number, letter, year)
		if err != nil {
			return nil
		}
		return ordin
	})
}
