package main

import (
	"context"
	"log/slog"
	"os"

	"processing/ai"
	"processing/config"
	"processing/db"
	"processing/fetch"
	"processing/processor"
)

type fetchWrapper struct{}

func (f fetchWrapper) Fetch(url string) (string, error) {
	return fetch.FetchAndExtract(url)
}

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
	fetcher := fetchWrapper{}

	items, err := db.ListPendingItems(ctx, pool)
	if err != nil {
		slog.Error("list pending", "error", err)
		os.Exit(1)
	}

	slog.Info("processing items", "count", len(items))

	for _, item := range items {
		if err := db.MarkProcessing(ctx, pool, item.ID); err != nil {
			slog.Error("mark processing", "id", item.ID, "error", err)
			continue
		}

		content, err := fetcher.Fetch(item.URL)
		if err != nil {
			slog.Warn("fetch failed", "id", item.ID, "url", item.URL, "error", err)
			db.MarkError(ctx, pool, item.ID, err.Error())
			continue
		}

		summary, err := aiClient.Complete(processor.SummaryPrompt(content))
		if err != nil {
			slog.Warn("summary failed", "id", item.ID, "error", err)
			db.MarkError(ctx, pool, item.ID, err.Error())
			continue
		}

		embText, err := aiClient.Complete(processor.EmbeddingTextPrompt(content, summary))
		if err != nil {
			slog.Warn("embedding text failed", "id", item.ID, "error", err)
			db.MarkError(ctx, pool, item.ID, err.Error())
			continue
		}

		if err := db.InsertProcessedItem(ctx, pool, db.ProcessedItemParams{
			IngestedItemID: item.ID,
			Summary:        summary,
			EmbeddingText:  embText,
		}); err != nil {
			slog.Error("insert processed", "id", item.ID, "error", err)
			db.MarkError(ctx, pool, item.ID, err.Error())
			continue
		}

		if err := db.MarkDone(ctx, pool, item.ID); err != nil {
			slog.Error("mark done", "id", item.ID, "error", err)
			continue
		}

		slog.Info("item processed", "id", item.ID, "url", item.URL)
	}
}
