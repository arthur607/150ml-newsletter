package main

import (
	"context"
	"log/slog"
	"os"

	"ingestion/config"
	"ingestion/db"
	"ingestion/scraper"
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

	sources, err := db.ListActiveSources(ctx, pool)
	if err != nil {
		slog.Error("list sources", "error", err)
		os.Exit(1)
	}

	rssScraper := scraper.NewRSSScraper()
	redditScraper := scraper.NewRedditScraper()
	hnScraper := scraper.NewHNScraper("")

	for _, source := range sources {
		var s scraper.Scraper
		switch source.Type {
		case "rss", "substack":
			s = rssScraper
		case "reddit":
			s = redditScraper
		case "hn":
			s = hnScraper
		default:
			slog.Warn("unsupported source type", "type", source.Type, "source", source.Name)
			continue
		}

		items, err := s.Scrape(source.URL)
		if err != nil {
			slog.Error("scrape failed", "source", source.Name, "error", err)
			continue
		}

		inserted, skipped := 0, 0
		for _, item := range items {
			ok, err := db.UpsertIngestedItem(ctx, pool, db.IngestParams{
				SourceID:    source.ID,
				ExternalID:  item.ExternalID,
				Title:       item.Title,
				URL:         item.URL,
				Author:      item.Author,
				PublishedAt: item.PublishedAt,
				RawContent:  item.RawContent,
			})
			if err != nil {
				slog.Error("upsert item", "source", source.Name, "url", item.URL, "error", err)
				continue
			}
			if ok {
				inserted++
			} else {
				skipped++
			}
		}
		slog.Info("source processed", "source", source.Name, "inserted", inserted, "skipped", skipped)
	}
}
