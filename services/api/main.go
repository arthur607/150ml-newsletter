package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"api/ai"
	"api/briefing"
	"api/config"
	"api/db"
	"api/handlers"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	aiClient := ai.NewTogetherClient(cfg.TogetherAPIKey, "")
	gen := briefing.NewGenerator(aiClient)

	go runBriefingCron(ctx, pool, gen)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	briefingH := handlers.NewBriefingHandler(pool)
	sourceH := handlers.NewSourceHandler(pool)
	itemH := handlers.NewItemHandler(pool)

	r.Get("/", briefingH.Latest)
	r.Get("/briefings/{date}", briefingH.ByDate)
	r.Get("/sources", sourceH.List)
	r.Post("/sources", sourceH.Add)
	r.Patch("/sources/{id}/toggle", sourceH.Toggle)
	r.Get("/items", itemH.List)

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func runBriefingCron(ctx context.Context, pool *pgxpool.Pool, gen *briefing.Generator) {
	for {
		now := time.Now()
		next := nextBriefingTime(now)
		slog.Info("next briefing generation", "at", next.Format(time.RFC3339))
		time.Sleep(time.Until(next))
		generateBriefing(ctx, pool, gen, now.Format("2006-01-02"))
	}
}

func nextBriefingTime(from time.Time) time.Time {
	targets := []int{1, 7, 13, 19}
	for _, h := range targets {
		candidate := time.Date(from.Year(), from.Month(), from.Day(), h, 30, 0, 0, from.Location())
		if candidate.After(from) {
			return candidate
		}
	}
	tomorrow := from.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 1, 30, 0, 0, from.Location())
}

func generateBriefing(ctx context.Context, pool *pgxpool.Pool, gen *briefing.Generator, date string) {
	clusters, err := db.ListClustersForDate(ctx, pool, date)
	if err != nil {
		slog.Error("list clusters", "date", date, "error", err)
		return
	}
	if len(clusters) == 0 {
		slog.Info("no clusters for briefing", "date", date)
		return
	}
	content, err := gen.Generate(clusters)
	if err != nil {
		slog.Error("generate briefing", "date", date, "error", err)
		return
	}
	if err := db.UpsertBriefing(ctx, pool, date, content); err != nil {
		slog.Error("upsert briefing", "date", date, "error", err)
		return
	}
	slog.Info("briefing generated", "date", date)
}
