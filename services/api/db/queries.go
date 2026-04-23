package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Briefing struct {
	ID      string
	Date    time.Time
	Content string
	Status  string
}

type Source struct {
	ID     string
	Type   string
	URL    string
	Name   string
	Active bool
}

type Item struct {
	ID           string
	SourceName   string
	Title        string
	URL          string
	Status       string
	ErrorMessage string
	IngestedAt   time.Time
}

type Cluster struct {
	ID    string
	Label string
	Items []ClusterItem
}

type ClusterItem struct {
	Title   string
	URL     string
	Summary string
}

func GetLatestBriefing(ctx context.Context, pool *pgxpool.Pool) (*Briefing, error) {
	var b Briefing
	err := pool.QueryRow(ctx,
		`SELECT id, date, content, status FROM briefings ORDER BY date DESC LIMIT 1`,
	).Scan(&b.ID, &b.Date, &b.Content, &b.Status)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func GetBriefingByDate(ctx context.Context, pool *pgxpool.Pool, date string) (*Briefing, error) {
	var b Briefing
	err := pool.QueryRow(ctx,
		`SELECT id, date, content, status FROM briefings WHERE date = $1`, date,
	).Scan(&b.ID, &b.Date, &b.Content, &b.Status)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func ListSources(ctx context.Context, pool *pgxpool.Pool) ([]Source, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, type, url, name, active FROM data_sources ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.Type, &s.URL, &s.Name, &s.Active); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func AddSource(ctx context.Context, pool *pgxpool.Pool, sourceType, url, name string) (Source, error) {
	var s Source
	err := pool.QueryRow(ctx,
		`INSERT INTO data_sources (type, url, name) VALUES ($1, $2, $3)
		 RETURNING id, type, url, name, active`,
		sourceType, url, name,
	).Scan(&s.ID, &s.Type, &s.URL, &s.Name, &s.Active)
	return s, err
}

func ToggleSource(ctx context.Context, pool *pgxpool.Pool, id string) (Source, error) {
	var s Source
	err := pool.QueryRow(ctx,
		`UPDATE data_sources SET active = NOT active WHERE id = $1
		 RETURNING id, type, url, name, active`, id,
	).Scan(&s.ID, &s.Type, &s.URL, &s.Name, &s.Active)
	return s, err
}

func ListItems(ctx context.Context, pool *pgxpool.Pool, sourceID, status string) ([]Item, error) {
	query := `SELECT ii.id, ds.name, ii.title, ii.url, ii.status,
	                 COALESCE(ii.error_message, ''), ii.ingested_at
	          FROM ingested_items ii
	          JOIN data_sources ds ON ds.id = ii.source_id
	          WHERE ($1 = '' OR ii.source_id::text = $1)
	            AND ($2 = '' OR ii.status = $2)
	          ORDER BY ii.ingested_at DESC LIMIT 200`
	rows, err := pool.Query(ctx, query, sourceID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.SourceName, &i.Title, &i.URL,
			&i.Status, &i.ErrorMessage, &i.IngestedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func ListClustersForDate(ctx context.Context, pool *pgxpool.Pool, date string) ([]Cluster, error) {
	rows, err := pool.Query(ctx,
		`SELECT c.id, c.label FROM clusters c WHERE c.briefing_date = $1`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clusters []Cluster
	for rows.Next() {
		var c Cluster
		if err := rows.Scan(&c.ID, &c.Label); err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, c := range clusters {
		itemRows, err := pool.Query(ctx, `
			SELECT ii.title, ii.url, pi.summary
			FROM processed_items pi
			JOIN ingested_items ii ON ii.id = pi.ingested_item_id
			WHERE pi.cluster_id = $1`, c.ID)
		if err != nil {
			return nil, err
		}
		defer itemRows.Close()
		for itemRows.Next() {
			var ci ClusterItem
			if err := itemRows.Scan(&ci.Title, &ci.URL, &ci.Summary); err != nil {
				return nil, err
			}
			clusters[i].Items = append(clusters[i].Items, ci)
		}
	}
	return clusters, nil
}

func UpsertBriefing(ctx context.Context, pool *pgxpool.Pool, date, content string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO briefings (date, content, status) VALUES ($1, $2, 'published')
		 ON CONFLICT (date) DO UPDATE SET content = EXCLUDED.content, status = 'published'`,
		date, content)
	return err
}
