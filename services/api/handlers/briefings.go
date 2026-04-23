package handlers

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yuin/goldmark"

	"api/db"
	"api/templates"
)

type BriefingHandler struct {
	pool *pgxpool.Pool
}

func NewBriefingHandler(pool *pgxpool.Pool) *BriefingHandler {
	return &BriefingHandler{pool: pool}
}

func (h *BriefingHandler) Latest(w http.ResponseWriter, r *http.Request) {
	briefing, err := db.GetLatestBriefing(r.Context(), h.pool)
	if err != nil {
		templates.NoBriefingPage().Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, "/briefings/"+briefing.Date.Format("2006-01-02"), http.StatusFound)
}

func (h *BriefingHandler) ByDate(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	briefing, err := db.GetBriefingByDate(r.Context(), h.pool, date)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(briefing.Content), &buf); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	templates.BriefingPage(date, template.HTML(buf.String())).Render(r.Context(), w)
}
