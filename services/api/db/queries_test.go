package db_test

import (
	"context"
	"os"
	"testing"

	"api/db"
)

func TestAddAndToggleSource(t *testing.T) {
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

	source, err := db.AddSource(ctx, pool, "rss", "https://test-api.example.com/feed", "Test API Source")
	if err != nil {
		t.Fatalf("add source: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM data_sources WHERE id = $1", source.ID)
	})

	if !source.Active {
		t.Error("new source should be active by default")
	}

	toggled, err := db.ToggleSource(ctx, pool, source.ID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if toggled.Active {
		t.Error("toggled source should be inactive")
	}
}
