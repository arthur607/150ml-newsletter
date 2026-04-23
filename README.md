# 150ml-newsletter

Personal content curation pipeline that ingests RSS/Reddit/Hacker News, clusters articles by topic, and delivers daily briefings via a web UI.

## How it works

Four services share one PostgreSQL instance. Data flows sequentially through the pipeline:

1. **Ingestion** (Go, cron) — scrapes configured sources, stores raw metadata with dedup
2. **Processing** (Go, cron) — fetches full articles, generates summaries and embedding text via TogetherAI
3. **ML** (Python, cron) — embeds and clusters articles by topic via UMAP + HDBSCAN
4. **API** (Go, always-on) — serves the briefing UI (templ + htmx), generates daily briefings via TogetherAI

## Quick start

```bash
# Start Postgres
make up

# Set required env vars
export DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable
export TOGETHER_AI_API_KEY=your_key_here

# Run migrations
make migrate

# Run ingestion
go run ./services/ingestion/main.go

# Run processing (needs real TOGETHER_AI_API_KEY)
go run ./services/processing/main.go

# Run ML service (needs real TOGETHER_AI_API_KEY)
cd services/ml && PYTHONPATH=. .venv/bin/python main.py

# Start API server
PORT=8080 go run ./services/api/main.go
```

Open `http://localhost:8080` to see the briefing UI. Add sources at `/sources`.

## Testing

```bash
# All Go tests (requires running Postgres + migrations)
DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  go test ./services/ingestion/... ./services/processing/... ./services/api/... -v

# ML tests
cd services/ml && PYTHONPATH=. .venv/bin/python -m pytest tests/ -v
```

## Project structure

```
packages/database/migrations/   goose SQL migrations
services/ingestion/              Go — RSS/Reddit/HN scraper
services/processing/             Go — article fetch + AI summarization
services/ml/                     Python — embeddings + clustering
services/api/                    Go — HTTP server, templ UI, briefing gen
```

## Tech stack

| Layer | Technology |
|-------|-----------|
| Core services | Go 1.25 |
| ML / clustering | Python 3.11+ |
| LLM / embeddings | TogetherAI API |
| Database | PostgreSQL 16 |
| Frontend | templ + htmx |
| Migrations | goose v3 |

## License

AGPL-3.0 — copyleft forte, uso comercial permitido desde que derivados sejam open source. See [LICENSE](LICENSE).
