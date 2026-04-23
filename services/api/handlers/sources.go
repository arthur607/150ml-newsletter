package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"api/db"
	"api/templates"
	"api/templates/fragments"
)

type SourceHandler struct {
	pool *pgxpool.Pool
}

func NewSourceHandler(pool *pgxpool.Pool) *SourceHandler {
	return &SourceHandler{pool: pool}
}

func (h *SourceHandler) List(w http.ResponseWriter, r *http.Request) {
	sources, err := db.ListSources(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	templates.SourcesPage(sources).Render(r.Context(), w)
}

func (h *SourceHandler) Add(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sourceType := r.FormValue("type")
	name := r.FormValue("name")
	url := r.FormValue("url")

	if sourceType == "" || name == "" || url == "" {
		http.Error(w, "type, name, and url are required", http.StatusBadRequest)
		return
	}

	if _, err := db.AddSource(r.Context(), h.pool, sourceType, url, name); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	sources, _ := db.ListSources(r.Context(), h.pool)
	templates.SourceList(sources).Render(r.Context(), w)
}

func (h *SourceHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	source, err := db.ToggleSource(r.Context(), h.pool, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	fragments.SourceRow(source).Render(r.Context(), w)
}
