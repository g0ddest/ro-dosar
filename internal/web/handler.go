package web

import (
	"embed"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Handler handles web requests
type Handler struct{}

// NewHandler creates a new web handler
func NewHandler() *Handler {
	return &Handler{}
}

// Index serves the SPA
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	// Only serve index.html for root path, let other paths fall through
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	content, err := templatesFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Error loading page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(content)
}

// Router returns the web router
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.Index)
	return mux
}
