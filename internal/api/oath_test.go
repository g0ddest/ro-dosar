package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ro-dosar/internal/repository"
)

func oathTime(s string) *string { return &s }

func TestGetDocument_OathForSolved(t *testing.T) {
	docRepo := newQueueDocument(t, "35573/RD/2022", time.Date(2022, 5, 1, 0, 0, 0, 0, time.UTC), "ART_11", true)
	oathRepo := &MockOathRepository{entry: &repository.OathEntryRecord{
		Number: 35573, Year: 2022,
		Date:    time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Time:    oathTime("09:00"),
		ListURL: "https://cetatenie.just.ro/wp-content/uploads/2026/08/tabel.pdf",
	}}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), &MockStatsRepository{},
		&MockAuditRepository{}, &MockOrdinRepository{}, oathRepo)

	rec := doDocumentRequest(t, handler, "35573", "RD", "2022")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Oath == nil || resp.Oath.Date != "2026-08-12" || resp.Oath.Time == nil || *resp.Oath.Time != "09:00" ||
		!strings.Contains(resp.Oath.ListURL, "tabel.pdf") {
		t.Errorf("wrong oath block: %+v", resp.Oath)
	}
}

func TestGetDocument_NoOathForUnsolved(t *testing.T) {
	docRepo := newQueueDocument(t, "400/RD/2024", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), "ART_11", false)
	oathRepo := &MockOathRepository{entry: &repository.OathEntryRecord{Number: 400, Year: 2024, Date: time.Now()}}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), &MockStatsRepository{},
		&MockAuditRepository{}, &MockOrdinRepository{}, oathRepo)

	rec := doDocumentRequest(t, handler, "400", "RD", "2024")

	if strings.Contains(rec.Body.String(), `"oath"`) {
		t.Error("unsolved documents must not carry an oath block")
	}
}

func TestGetDocument_OathLookupFailureNonFatal(t *testing.T) {
	docRepo := newQueueDocument(t, "101/A/2021", time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), "ART_10", true)
	oathRepo := &MockOathRepository{entryErr: context.DeadlineExceeded}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), &MockStatsRepository{},
		&MockAuditRepository{}, &MockOrdinRepository{}, oathRepo)

	rec := doDocumentRequest(t, handler, "101", "A", "2021")

	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"oath"`) {
		t.Errorf("oath failure must not fail the request: %d", rec.Code)
	}
}
