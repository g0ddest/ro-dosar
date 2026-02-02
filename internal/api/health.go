package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ro-dosar/internal/infrastructure/postgres"
)

// HealthHandler handles health and readiness checks
type HealthHandler struct {
	db           *postgres.DB
	temporalHost string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *postgres.DB, temporalHost string) *HealthHandler {
	return &HealthHandler{
		db:           db,
		temporalHost: temporalHost,
	}
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyResponse represents a readiness check response
type ReadyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Health handles GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

// Ready handles GET /ready
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	response := ReadyResponse{
		Status: "ok",
		Checks: make(map[string]string),
	}

	// Check database
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		response.Status = "degraded"
		response.Checks["database"] = "error: " + err.Error()
	} else {
		response.Checks["database"] = "ok"
	}

	// Check Temporal (basic TCP check would be done here)
	// For now, just mark as ok if configured
	if h.temporalHost != "" {
		response.Checks["temporal"] = "ok"
	} else {
		response.Checks["temporal"] = "not configured"
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(response)
}

// Metrics returns the Prometheus metrics handler
func (h *HealthHandler) Metrics() http.Handler {
	return promhttp.Handler()
}

// Router returns the health router
func (h *HealthHandler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/ready", h.Ready)
	mux.Handle("/metrics", h.Metrics())
	return mux
}
