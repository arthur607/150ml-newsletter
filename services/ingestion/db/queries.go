package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DataSource struct {
	ID   string
	Type string
	URL  string
	Name string
}

type IngestParams struct {
	SourceID    string
	ExternalID  string
	Title       string
	URL         string
	Author      string
	PublishedAt *time.Time
	RawContent  string
}

func ListActiveSources(ctx context.Context, pool *pgxpool.Pool) ([]DataSource, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, type, url, name FROM data_sources WHERE active = true`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []DataSource
	for rows.Next() {
		var s DataSource
		if err := rows.Scan(&s.ID, &s.Type, &s.URL, &s.Name); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func UpsertIngestedItem(ctx context.Context, pool *pgxpool.Pool, p IngestParams) (bool, error) {
	tag, err := pool.Exec(ctx, `
		INSERT INTO ingested_items
		  (source_id, external_id, title, url, author, published_at, raw_content)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (source_id, external_id) DO NOTHING`,
		p.SourceID, p.ExternalID, p.Title, p.URL, p.Author, p.PublishedAt, p.RawContent,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
