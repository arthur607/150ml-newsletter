package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"ingestion/db"
)

func TestUpsertIngestedItem_Dedup(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var sourceID string
	err = pool.QueryRow(ctx,
		`INSERT INTO data_sources (type, url, name) VALUES ('rss', 'https://test.example.com/feed', 'Test') RETURNING id`,
	).Scan(&sourceID)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM ingested_items WHERE source_id = $1", sourceID)
		pool.Exec(ctx, "DELETE FROM data_sources WHERE id = $1", sourceID)
	})

	now := time.Now()
	item := db.IngestParams{
		SourceID:    sourceID,
		ExternalID:  "ext-001",
		Title:       "Test Article",
		URL:         "https://test.example.com/article-001",
		Author:      "Author",
		PublishedAt: &now,
		RawContent:  "some content",
	}

	inserted, err := db.UpsertIngestedItem(ctx, pool, item)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !inserted {
		t.Error("expected first insert to return true")
	}

	inserted, err = db.UpsertIngestedItem(ctx, pool, item)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if inserted {
		t.Error("expected duplicate insert to return false")
	}
}
