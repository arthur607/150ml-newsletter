package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PendingItem struct {
	ID         string
	URL        string
	RetryCount int
}

func ListPendingItems(ctx context.Context, pool *pgxpool.Pool) ([]PendingItem, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, url, retry_count FROM ingested_items
		 WHERE status = 'pending' AND retry_count < 3
		 ORDER BY ingested_at ASC LIMIT 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PendingItem
	for rows.Next() {
		var i PendingItem
		if err := rows.Scan(&i.ID, &i.URL, &i.RetryCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func MarkProcessing(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx,
		`UPDATE ingested_items SET status = 'processing' WHERE id = $1`, id)
	return err
}

func MarkDone(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx,
		`UPDATE ingested_items SET status = 'done' WHERE id = $1`, id)
	return err
}

func MarkError(ctx context.Context, pool *pgxpool.Pool, id, msg string) error {
	_, err := pool.Exec(ctx,
		`UPDATE ingested_items
		 SET status = CASE WHEN retry_count + 1 >= 3 THEN 'error' ELSE 'pending' END,
		     retry_count = retry_count + 1,
		     error_message = $2
		 WHERE id = $1`, id, msg)
	return err
}

type ProcessedItemParams struct {
	IngestedItemID string
	Summary        string
	EmbeddingText  string
}

func InsertProcessedItem(ctx context.Context, pool *pgxpool.Pool, p ProcessedItemParams) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO processed_items (ingested_item_id, summary, embedding_text, processed_at)
		 VALUES ($1, $2, $3, $4)`,
		p.IngestedItemID, p.Summary, p.EmbeddingText, time.Now())
	return err
}
