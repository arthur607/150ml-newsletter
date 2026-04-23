package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"api/db"
	"api/templates"
)

type ItemHandler struct {
	pool *pgxpool.Pool
}

func NewItemHandler(pool *pgxpool.Pool) *ItemHandler {
	return &ItemHandler{pool: pool}
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("source")
	status := r.URL.Query().Get("status")

	items, err := db.ListItems(r.Context(), h.pool, sourceID, status)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	sources, _ := db.ListSources(r.Context(), h.pool)

	if r.Header.Get("HX-Request") == "true" {
		templates.ItemsList(items).Render(r.Context(), w)
		return
	}
	templates.ItemsPage(items, sources).Render(r.Context(), w)
}
