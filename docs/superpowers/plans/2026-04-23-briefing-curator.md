# Briefing Curator — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a personal content curation pipeline that ingests RSS/Reddit/HN, clusters articles by topic via ML, and serves daily briefings via a templ+htmx web UI on Railway.

**Architecture:** Four services (ingestion, processing, ml, api) share one PostgreSQL instance on Railway. Each service runs as an independent Railway cron job except the API which stays always online. Data flows sequentially through the pipeline via `ingested_items.status` flags and `processed_items.cluster_id`.

**Tech Stack:** Go 1.22 (ingestion, processing, api) · Python 3.11 (ml) · pgx/v5 (postgres driver) · chi v5 (http router) · templ + htmx (UI) · goldmark (markdown) · TogetherAI API (LLM + embeddings) · goose v3 (migrations) · gofeed (RSS) · goquery (HTML extraction) · Docker Compose (local dev)

---

## File Map

```
/
├── go.work                                    # Go workspace linking all Go services
├── docker-compose.yml                         # Local Postgres for dev/test
├── Makefile                                   # migrate, test, generate
├── .env.example                               # All required env vars documented
│
├── packages/database/
│   └── migrations/
│       └── 001_initial.sql                    # All 5 tables
│
├── services/ingestion/
│   ├── go.mod                                 # module ingestion
│   ├── main.go                                # entrypoint: load sources, run scrapers, exit
│   ├── config/config.go                       # DB_URL, env loading
│   ├── db/db.go                               # pgx pool constructor
│   ├── db/queries.go                          # ListActiveSources, UpsertIngestedItem
│   └── scraper/
│       ├── scraper.go                         # Item struct + Scraper interface
│       ├── rss.go                             # RSS/Atom/Substack via gofeed
│       ├── reddit.go                          # Reddit JSON API client
│       └── hn.go                              # HN Firebase API client
│
├── services/processing/
│   ├── go.mod                                 # module processing
│   ├── main.go                                # entrypoint: fetch pending items, process, exit
│   ├── config/config.go                       # DB_URL, TOGETHER_API_KEY
│   ├── db/db.go                               # pgx pool constructor
│   ├── db/queries.go                          # ListPendingItems, MarkProcessing, MarkDone, MarkError, InsertProcessedItem
│   ├── fetch/fetch.go                         # HTTP fetch + goquery HTML-to-text
│   ├── ai/together.go                         # TogetherAI LLM client (summary + embedding_text)
│   └── processor/processor.go                # orchestrates fetch→AI→DB with 3x retry
│
├── services/ml/
│   ├── pyproject.toml                         # uv project: psycopg, umap-learn, hdbscan, requests, python-dotenv
│   ├── main.py                                # entrypoint: run pipeline, exit
│   ├── config.py                              # DB_URL, TOGETHER_API_KEY from env
│   ├── db.py                                  # psycopg connection + queries
│   ├── embeddings.py                          # TogetherAI embeddings API
│   ├── clustering.py                          # UMAP + HDBSCAN
│   ├── labeler.py                             # TogetherAI LLM → cluster label
│   └── pipeline.py                            # orchestrates db→embed→cluster→label→save
│
└── services/api/
    ├── go.mod                                 # module api
    ├── main.go                                # entrypoint: HTTP server + brief-gen cron
    ├── config/config.go                       # PORT, DB_URL, TOGETHER_API_KEY
    ├── db/db.go                               # pgx pool constructor
    ├── db/queries.go                          # GetLatestBriefing, GetBriefingByDate, ListSources, AddSource, ToggleSource, ListItems, UpsertBriefing, ListClustersForDate
    ├── briefing/generator.go                  # TogetherAI: cluster analysis + final synthesis
    ├── ai/together.go                         # TogetherAI LLM client (same pattern as processing)
    ├── handlers/briefings.go                  # GET /, GET /briefings/:date
    ├── handlers/sources.go                    # GET /sources, POST /sources, PATCH /sources/:id/toggle
    ├── handlers/items.go                      # GET /items?source=&status=
    └── templates/
        ├── layout.templ                       # base HTML shell + htmx CDN
        ├── briefing.templ                     # full briefing page
        ├── sources.templ                      # source manager page
        ├── items.templ                        # items list page
        └── fragments/
            ├── source_row.templ               # htmx swap fragment for one source row
            └── items_list.templ               # htmx swap fragment for filtered items
```

---

## Group 1: Foundation

### Task 1: Monorepo scaffold

**Files:**
- Create: `docker-compose.yml`
- Create: `Makefile`
- Create: `.env.example`
- Create: `go.work`

- [ ] **Step 1: Create docker-compose.yml**

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: curator
      POSTGRES_PASSWORD: curator
      POSTGRES_DB: curator
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U curator"]
      interval: 5s
      timeout: 5s
      retries: 5
```

- [ ] **Step 2: Create .env.example**

```bash
# .env.example
DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable
TOGETHER_API_KEY=your_key_here
PORT=8080
```

- [ ] **Step 3: Create Makefile**

```makefile
# Makefile
.PHONY: up down migrate test generate

up:
	docker compose up -d

down:
	docker compose down

migrate:
	goose -dir packages/database/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir packages/database/migrations postgres "$(DATABASE_URL)" down

generate:
	go generate ./...

test:
	docker compose up -d
	sleep 2
	DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
	  go test ./services/ingestion/... ./services/processing/... ./services/api/... -v
	cd services/ml && python -m pytest -v

test-go:
	DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
	  go test ./services/ingestion/... ./services/processing/... ./services/api/... -v
```

- [ ] **Step 4: Start Docker Compose and verify Postgres is up**

```bash
docker compose up -d
docker compose ps
```
Expected: `postgres` service shows `healthy`.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml Makefile .env.example
git commit -m "chore: add monorepo scaffold — docker-compose, Makefile, env example"
```

---

### Task 2: Database migrations

**Files:**
- Create: `packages/database/migrations/001_initial.sql`

- [ ] **Step 1: Install goose globally**

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

- [ ] **Step 2: Write migration file**

```sql
-- packages/database/migrations/001_initial.sql
-- +goose Up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE data_sources (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type       TEXT NOT NULL CHECK (type IN ('rss','reddit','hn','substack','twitter','youtube')),
  url        TEXT NOT NULL,
  name       TEXT NOT NULL,
  active     BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingested_items (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id     UUID NOT NULL REFERENCES data_sources(id),
  external_id   TEXT NOT NULL,
  title         TEXT NOT NULL,
  url           TEXT NOT NULL,
  author        TEXT,
  published_at  TIMESTAMPTZ,
  raw_content   TEXT,
  status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','processing','done','error')),
  retry_count   INTEGER NOT NULL DEFAULT 0,
  error_message TEXT,
  ingested_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (source_id, external_id),
  UNIQUE (url)
);

CREATE TABLE clusters (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  briefing_date DATE NOT NULL,
  label         TEXT NOT NULL,
  item_count    INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE processed_items (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ingested_item_id UUID NOT NULL REFERENCES ingested_items(id),
  summary          TEXT NOT NULL,
  embedding_text   TEXT NOT NULL,
  cluster_id       UUID REFERENCES clusters(id),
  processed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE briefings (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  date       DATE NOT NULL UNIQUE,
  content    TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE briefings;
DROP TABLE processed_items;
DROP TABLE clusters;
DROP TABLE ingested_items;
DROP TABLE data_sources;
```

- [ ] **Step 3: Run migration against local Postgres**

```bash
export DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable
make migrate
```
Expected output:
```
OK    001_initial.sql
goose: successfully migrated database to version: 1
```

- [ ] **Step 4: Verify tables exist**

```bash
docker compose exec postgres psql -U curator -c "\dt"
```
Expected: lists `briefings`, `clusters`, `data_sources`, `ingested_items`, `processed_items`.

- [ ] **Step 5: Commit**

```bash
git add packages/database/migrations/001_initial.sql
git commit -m "feat(db): add initial schema migration — all 5 tables"
```

---

### Task 3: Go workspace

**Files:**
- Create: `go.work`
- Create: `services/ingestion/go.mod`
- Create: `services/processing/go.mod`
- Create: `services/api/go.mod`

- [ ] **Step 1: Initialize Go modules for each service**

```bash
cd services/ingestion && go mod init ingestion && cd ../..
cd services/processing && go mod init processing && cd ../..
cd services/api && go mod init api && cd ../..
```

- [ ] **Step 2: Create go.work at repo root**

```bash
go work init ./services/ingestion ./services/processing ./services/api
```

This creates:
```
go.work
go 1.22

use (
    ./services/ingestion
    ./services/processing
    ./services/api
)
```

- [ ] **Step 3: Verify workspace**

```bash
go work sync
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add go.work services/ingestion/go.mod services/processing/go.mod services/api/go.mod
git commit -m "chore: initialize Go workspace with three service modules"
```

---

## Group 2: Ingestion Service

### Task 4: Scraper interface + RSS/Substack scraper

**Files:**
- Create: `services/ingestion/scraper/scraper.go`
- Create: `services/ingestion/scraper/rss.go`
- Create: `services/ingestion/scraper/rss_test.go`

- [ ] **Step 1: Add gofeed dependency**

```bash
cd services/ingestion && go get github.com/mmcdole/gofeed
```

- [ ] **Step 2: Write the Item struct and Scraper interface**

```go
// services/ingestion/scraper/scraper.go
package scraper

import "time"

type Item struct {
	ExternalID  string
	Title       string
	URL         string
	Author      string
	PublishedAt *time.Time
	RawContent  string // excerpt from feed, may be empty
}

type Scraper interface {
	Scrape(sourceURL string) ([]Item, error)
}
```

- [ ] **Step 3: Write the failing RSS test**

```go
// services/ingestion/scraper/rss_test.go
package scraper_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ingestion/scraper"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Test Article</title>
      <link>https://example.com/article-1</link>
      <guid>article-1</guid>
      <author>Jane Doe</author>
      <description>Short excerpt here.</description>
      <pubDate>Wed, 23 Apr 2026 10:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

func TestRSSScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	s := scraper.NewRSSScraper()
	items, err := s.Scrape(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	got := items[0]
	if got.Title != "Test Article" {
		t.Errorf("title: got %q, want %q", got.Title, "Test Article")
	}
	if got.URL != "https://example.com/article-1" {
		t.Errorf("url: got %q", got.URL)
	}
	if got.ExternalID != "article-1" {
		t.Errorf("externalID: got %q", got.ExternalID)
	}
	if got.Author != "Jane Doe" {
		t.Errorf("author: got %q", got.Author)
	}
	if got.PublishedAt == nil {
		t.Error("published_at should not be nil")
	}
}
```

- [ ] **Step 4: Run test — expect FAIL**

```bash
cd services/ingestion && go test ./scraper/... -run TestRSSScraper_Scrape -v
```
Expected: `FAIL` — `scraper.NewRSSScraper undefined`.

- [ ] **Step 5: Implement RSScraper**

```go
// services/ingestion/scraper/rss.go
package scraper

import "github.com/mmcdole/gofeed"

type RSSScraper struct {
	parser *gofeed.Parser
}

func NewRSSScraper() *RSSScraper {
	return &RSSScraper{parser: gofeed.NewParser()}
}

func (s *RSSScraper) Scrape(sourceURL string) ([]Item, error) {
	feed, err := s.parser.ParseURL(sourceURL)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(feed.Items))
	for _, fi := range feed.Items {
		item := Item{
			ExternalID: fi.GUID,
			Title:      fi.Title,
			URL:        fi.Link,
			RawContent: fi.Description,
		}
		if fi.Author != nil {
			item.Author = fi.Author.Name
		}
		if fi.PublishedParsed != nil {
			t := *fi.PublishedParsed
			item.PublishedAt = &t
		}
		if item.ExternalID == "" {
			item.ExternalID = fi.Link
		}
		items = append(items, item)
	}
	return items, nil
}
```

- [ ] **Step 6: Run test — expect PASS**

```bash
cd services/ingestion && go test ./scraper/... -run TestRSSScraper_Scrape -v
```
Expected: `PASS`.

- [ ] **Step 7: Commit**

```bash
git add services/ingestion/scraper/
git commit -m "feat(ingestion): add scraper interface and RSS/Substack scraper"
```

---

### Task 5: Reddit scraper

**Files:**
- Create: `services/ingestion/scraper/reddit.go`
- Create: `services/ingestion/scraper/reddit_test.go`

- [ ] **Step 1: Write the failing Reddit test**

```go
// services/ingestion/scraper/reddit_test.go
package scraper_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ingestion/scraper"
)

const sampleRedditJSON = `{
  "data": {
    "children": [
      {
        "data": {
          "id": "abc123",
          "title": "Interesting Go Post",
          "url": "https://example.com/go-post",
          "author": "gopher42",
          "selftext": "Some content here",
          "created_utc": 1745395200
        }
      }
    ]
  }
}`

func TestRedditScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("Reddit requires User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleRedditJSON))
	}))
	defer srv.Close()

	s := scraper.NewRedditScraper()
	items, err := s.Scrape(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	got := items[0]
	if got.ExternalID != "abc123" {
		t.Errorf("externalID: got %q", got.ExternalID)
	}
	if got.Title != "Interesting Go Post" {
		t.Errorf("title: got %q", got.Title)
	}
	if got.Author != "gopher42" {
		t.Errorf("author: got %q", got.Author)
	}
	if got.PublishedAt == nil {
		t.Error("published_at should not be nil")
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd services/ingestion && go test ./scraper/... -run TestRedditScraper_Scrape -v
```
Expected: `FAIL` — `scraper.NewRedditScraper undefined`.

- [ ] **Step 3: Implement RedditScraper**

```go
// services/ingestion/scraper/reddit.go
package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type RedditScraper struct {
	client *http.Client
}

func NewRedditScraper() *RedditScraper {
	return &RedditScraper{client: &http.Client{Timeout: 10 * time.Second}}
}

type redditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				ID         string  `json:"id"`
				Title      string  `json:"title"`
				URL        string  `json:"url"`
				Author     string  `json:"author"`
				Selftext   string  `json:"selftext"`
				CreatedUTC float64 `json:"created_utc"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func (s *RedditScraper) Scrape(sourceURL string) ([]Item, error) {
	req, err := http.NewRequest("GET", sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "briefing-curator/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit API returned %d", resp.StatusCode)
	}

	var listing redditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		d := child.Data
		t := time.Unix(int64(d.CreatedUTC), 0)
		items = append(items, Item{
			ExternalID:  d.ID,
			Title:       d.Title,
			URL:         d.URL,
			Author:      d.Author,
			RawContent:  d.Selftext,
			PublishedAt: &t,
		})
	}
	return items, nil
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
cd services/ingestion && go test ./scraper/... -run TestRedditScraper_Scrape -v
```
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/scraper/reddit.go services/ingestion/scraper/reddit_test.go
git commit -m "feat(ingestion): add Reddit scraper"
```

---

### Task 6: Hacker News scraper

**Files:**
- Create: `services/ingestion/scraper/hn.go`
- Create: `services/ingestion/scraper/hn_test.go`

- [ ] **Step 1: Write the failing HN test**

```go
// services/ingestion/scraper/hn_test.go
package scraper_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ingestion/scraper"
)

func TestHNScraper_Scrape(t *testing.T) {
	topStoriesJSON := `[111, 222]`
	story111 := `{"id":111,"title":"Cool HN Story","url":"https://example.com/hn-story","by":"hnuser","time":1745395200,"score":42}`
	story222 := `{"id":222,"title":"Ask HN: Something","url":"","by":"other","time":1745391600,"score":10,"text":"Ask content"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/topstories.json":
			fmt.Fprint(w, topStoriesJSON)
		case "/item/111.json":
			fmt.Fprint(w, story111)
		case "/item/222.json":
			fmt.Fprint(w, story222)
		}
	}))
	defer srv.Close()

	s := scraper.NewHNScraper(srv.URL)
	items, err := s.Scrape("topstories")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ExternalID != "111" {
		t.Errorf("externalID: got %q", items[0].ExternalID)
	}
	if items[0].Title != "Cool HN Story" {
		t.Errorf("title: got %q", items[0].Title)
	}
	// story 222 has no URL — falls back to HN item URL
	if items[1].URL == "" {
		t.Error("URL should fall back to HN item URL when post URL is empty")
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd services/ingestion && go test ./scraper/... -run TestHNScraper_Scrape -v
```
Expected: `FAIL` — `scraper.NewHNScraper undefined`.

- [ ] **Step 3: Implement HNScraper**

```go
// services/ingestion/scraper/hn.go
package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const defaultHNBase = "https://hacker-news.firebaseio.com/v0"

type HNScraper struct {
	baseURL string
	client  *http.Client
}

func NewHNScraper(baseURL string) *HNScraper {
	if baseURL == "" {
		baseURL = defaultHNBase
	}
	return &HNScraper{baseURL: baseURL, client: &http.Client{Timeout: 10 * time.Second}}
}

type hnItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	By    string `json:"by"`
	Time  int64  `json:"time"`
	Text  string `json:"text"`
}

func (s *HNScraper) Scrape(sourceURL string) ([]Item, error) {
	ids, err := s.fetchIDs(sourceURL)
	if err != nil {
		return nil, err
	}
	if len(ids) > 30 {
		ids = ids[:30]
	}
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		item, err := s.fetchItem(id)
		if err != nil {
			continue
		}
		items = append(items, *item)
	}
	return items, nil
}

func (s *HNScraper) fetchIDs(kind string) ([]int, error) {
	url := fmt.Sprintf("%s/%s.json", s.baseURL, kind)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var ids []int
	return ids, json.NewDecoder(resp.Body).Decode(&ids)
}

func (s *HNScraper) fetchItem(id int) (*Item, error) {
	url := fmt.Sprintf("%s/item/%d.json", s.baseURL, id)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var hi hnItem
	if err := json.NewDecoder(resp.Body).Decode(&hi); err != nil {
		return nil, err
	}
	itemURL := hi.URL
	if itemURL == "" {
		itemURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", hi.ID)
	}
	t := time.Unix(hi.Time, 0)
	return &Item{
		ExternalID:  strconv.Itoa(hi.ID),
		Title:       hi.Title,
		URL:         itemURL,
		Author:      hi.By,
		RawContent:  hi.Text,
		PublishedAt: &t,
	}, nil
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
cd services/ingestion && go test ./scraper/... -run TestHNScraper_Scrape -v
```
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add services/ingestion/scraper/hn.go services/ingestion/scraper/hn_test.go
git commit -m "feat(ingestion): add Hacker News scraper"
```

---

### Task 7: Ingestion DB layer + dedup integration test

**Files:**
- Create: `services/ingestion/config/config.go`
- Create: `services/ingestion/db/db.go`
- Create: `services/ingestion/db/queries.go`
- Create: `services/ingestion/db/queries_test.go`

- [ ] **Step 1: Add pgx dependency**

```bash
cd services/ingestion && go get github.com/jackc/pgx/v5
```

- [ ] **Step 2: Write config.go**

```go
// services/ingestion/config/config.go
package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL string
}

func Load() Config {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	return Config{DatabaseURL: url}
}
```

- [ ] **Step 3: Write db.go**

```go
// services/ingestion/db/db.go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
```

- [ ] **Step 4: Write the failing dedup integration test**

```go
// services/ingestion/db/queries_test.go
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

	// Insert a test source
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

	// First insert should succeed
	inserted, err := db.UpsertIngestedItem(ctx, pool, item)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !inserted {
		t.Error("expected first insert to return true")
	}

	// Second insert of same item should be silently skipped
	inserted, err = db.UpsertIngestedItem(ctx, pool, item)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if inserted {
		t.Error("expected duplicate insert to return false")
	}
}
```

- [ ] **Step 5: Run test — expect FAIL**

```bash
cd services/ingestion && DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  go test ./db/... -run TestUpsertIngestedItem_Dedup -v
```
Expected: `FAIL` — `db.UpsertIngestedItem undefined`.

- [ ] **Step 6: Implement queries.go**

```go
// services/ingestion/db/queries.go
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

// UpsertIngestedItem inserts the item if it does not already exist.
// Returns true if inserted, false if skipped due to deduplication.
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
```

- [ ] **Step 7: Run test — expect PASS**

```bash
cd services/ingestion && DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  go test ./db/... -run TestUpsertIngestedItem_Dedup -v
```
Expected: `PASS`.

- [ ] **Step 8: Commit**

```bash
git add services/ingestion/config/ services/ingestion/db/
git commit -m "feat(ingestion): add config, DB connection, and dedup upsert"
```

---

### Task 8: Ingestion main entrypoint

**Files:**
- Create: `services/ingestion/main.go`

- [ ] **Step 1: Write main.go**

```go
// services/ingestion/main.go
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
```

- [ ] **Step 2: Build to verify it compiles**

```bash
cd services/ingestion && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Seed a test source and run manually**

```bash
docker compose exec postgres psql -U curator -c \
  "INSERT INTO data_sources (type, url, name) VALUES ('hn', 'topstories', 'Hacker News Top')"

DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  go run ./services/ingestion/main.go
```
Expected: JSON log lines showing items inserted.

- [ ] **Step 4: Commit**

```bash
git add services/ingestion/main.go
git commit -m "feat(ingestion): add main entrypoint wiring all scrapers"
```

---

## Group 3: Processing Service

### Task 9: HTML-to-text fetcher

**Files:**
- Create: `services/processing/fetch/fetch.go`
- Create: `services/processing/fetch/fetch_test.go`

- [ ] **Step 1: Add goquery dependency**

```bash
cd services/processing && go get github.com/PuerkitoBio/goquery
```

- [ ] **Step 2: Write the failing fetch test**

```go
// services/processing/fetch/fetch_test.go
package fetch_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"processing/fetch"
)

const sampleHTML = `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
  <nav>Navigation garbage</nav>
  <article>
    <h1>Real Article Title</h1>
    <p>First paragraph of content.</p>
    <p>Second paragraph of content.</p>
  </article>
  <footer>Footer garbage</footer>
</body>
</html>`

func TestFetchAndExtract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sampleHTML))
	}))
	defer srv.Close()

	text, err := fetch.FetchAndExtract(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "First paragraph") {
		t.Errorf("expected article content in output, got: %q", text)
	}
	if strings.Contains(text, "Navigation garbage") {
		t.Errorf("nav content should be stripped, got: %q", text)
	}
	if strings.Contains(text, "Footer garbage") {
		t.Errorf("footer content should be stripped, got: %q", text)
	}
	if len(text) < 10 {
		t.Errorf("extracted text too short: %q", text)
	}
}
```

- [ ] **Step 3: Run test — expect FAIL**

```bash
cd services/processing && go test ./fetch/... -v
```
Expected: `FAIL` — `fetch.FetchAndExtract undefined`.

- [ ] **Step 4: Implement fetch.go**

```go
// services/processing/fetch/fetch.go
package fetch

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var client = &http.Client{Timeout: 15 * time.Second}

// FetchAndExtract fetches a URL and returns readable text content.
// Removes nav, footer, header, script, and style elements.
func FetchAndExtract(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "briefing-curator/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	doc.Find("nav, footer, header, script, style, aside").Remove()

	var sb strings.Builder
	doc.Find("article, main, body").First().Find("p, h1, h2, h3, h4, li").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	})

	result := strings.TrimSpace(sb.String())
	if result == "" {
		result = strings.TrimSpace(doc.Text())
	}
	return result, nil
}
```

- [ ] **Step 5: Run test — expect PASS**

```bash
cd services/processing && go test ./fetch/... -v
```
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add services/processing/fetch/
git commit -m "feat(processing): add HTML fetch and text extraction"
```

---

### Task 10: TogetherAI client (Processing)

**Files:**
- Create: `services/processing/ai/together.go`
- Create: `services/processing/ai/together_test.go`

- [ ] **Step 1: Write the failing TogetherAI client test**

```go
// services/processing/ai/together_test.go
package ai_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"processing/ai"
)

func TestTogetherClient_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "Generated summary."}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := ai.NewTogetherClient("test-key", srv.URL)
	result, err := client.Complete("Summarize this: hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Generated summary." {
		t.Errorf("got %q", result)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd services/processing && go test ./ai/... -v
```
Expected: `FAIL` — `ai.NewTogetherClient undefined`.

- [ ] **Step 3: Implement together.go**

```go
// services/processing/ai/together.go
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.together.xyz/v1"

type TogetherClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewTogetherClient(apiKey, baseURL string) *TogetherClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &TogetherClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Complete sends a user prompt and returns the assistant's text response.
// Uses meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo for fast, cheap inference.
func (c *TogetherClient) Complete(prompt string) (string, error) {
	body := chatRequest{
		Model:     "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo",
		Messages:  []chatMessage{{Role: "user", Content: prompt}},
		MaxTokens: 512,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("together API returned %d", resp.StatusCode)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
cd services/processing && go test ./ai/... -v
```
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add services/processing/ai/
git commit -m "feat(processing): add TogetherAI LLM client"
```

---

### Task 11: Processor with retry + DB layer

**Files:**
- Create: `services/processing/db/db.go`
- Create: `services/processing/db/queries.go`
- Create: `services/processing/config/config.go`
- Create: `services/processing/processor/processor.go`
- Create: `services/processing/processor/processor_test.go`

- [ ] **Step 1: Add pgx dependency**

```bash
cd services/processing && go get github.com/jackc/pgx/v5
```

- [ ] **Step 2: Write config.go and db.go (same pattern as ingestion)**

```go
// services/processing/config/config.go
package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL    string
	TogetherAPIKey string
}

func Load() Config {
	dbURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if dbURL == "" || apiKey == "" {
		slog.Error("DATABASE_URL and TOGETHER_API_KEY are required")
		os.Exit(1)
	}
	return Config{DatabaseURL: dbURL, TogetherAPIKey: apiKey}
}
```

```go
// services/processing/db/db.go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
```

- [ ] **Step 3: Write db/queries.go**

```go
// services/processing/db/queries.go
package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PendingItem struct {
	ID          string
	URL         string
	RetryCount  int
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
```

- [ ] **Step 4: Write processor_test.go with mocked AI**

```go
// services/processing/processor/processor_test.go
package processor_test

import (
	"testing"

	"processing/processor"
)

type mockAI struct {
	responses map[string]string
	calls     int
}

func (m *mockAI) Complete(prompt string) (string, error) {
	m.calls++
	return "mocked response", nil
}

type mockFetcher struct {
	content string
	err     error
}

func (f *mockFetcher) Fetch(url string) (string, error) {
	return f.content, f.err
}

func TestBuildPrompts(t *testing.T) {
	content := "This is article content about Go programming."
	summaryPrompt := processor.SummaryPrompt(content)
	if len(summaryPrompt) < 20 {
		t.Error("summary prompt too short")
	}
	embeddingPrompt := processor.EmbeddingTextPrompt(content, "mock summary")
	if len(embeddingPrompt) < 20 {
		t.Error("embedding text prompt too short")
	}
}
```

- [ ] **Step 5: Run test — expect FAIL**

```bash
cd services/processing && go test ./processor/... -v
```
Expected: `FAIL` — `processor.SummaryPrompt undefined`.

- [ ] **Step 6: Implement processor.go**

```go
// services/processing/processor/processor.go
package processor

import "fmt"

// Completer is satisfied by ai.TogetherClient.
type Completer interface {
	Complete(prompt string) (string, error)
}

// Fetcher is satisfied by fetch.FetchAndExtract (wrapped as a struct).
type Fetcher interface {
	Fetch(url string) (string, error)
}

func SummaryPrompt(content string) string {
	return fmt.Sprintf(
		"Summarize the following article in 150 words or fewer. "+
			"Focus on the main ideas and key takeaways. "+
			"Return only the summary text, no preamble.\n\n%s",
		truncate(content, 4000),
	)
}

func EmbeddingTextPrompt(content, summary string) string {
	return fmt.Sprintf(
		"Create a short structured representation of this article for semantic search. "+
			"Include: main topic, key entities, core argument. "+
			"Keep it under 100 words. Return only the text.\n\nSummary: %s\n\nContent excerpt: %s",
		summary,
		truncate(content, 1000),
	)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
```

- [ ] **Step 7: Run test — expect PASS**

```bash
cd services/processing && go test ./processor/... -v
```
Expected: `PASS`.

- [ ] **Step 8: Commit**

```bash
git add services/processing/config/ services/processing/db/ services/processing/processor/
git commit -m "feat(processing): add DB layer, retry logic, and processor prompt builders"
```

---

### Task 12: Processing Service main entrypoint

**Files:**
- Create: `services/processing/main.go`

- [ ] **Step 1: Write main.go**

```go
// services/processing/main.go
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

// fetchWrapper adapts fetch.FetchAndExtract to the processor.Fetcher interface.
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

	ai := ai.NewTogetherClient(cfg.TogetherAPIKey, "")
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

		summary, err := ai.Complete(processor.SummaryPrompt(content))
		if err != nil {
			slog.Warn("summary failed", "id", item.ID, "error", err)
			db.MarkError(ctx, pool, item.ID, err.Error())
			continue
		}

		embText, err := ai.Complete(processor.EmbeddingTextPrompt(content, summary))
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
```

- [ ] **Step 2: Build to verify compilation**

```bash
cd services/processing && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add services/processing/main.go
git commit -m "feat(processing): add main entrypoint orchestrating fetch+AI+DB"
```

---

## Group 4: ML Service

### Task 13: Python project setup + DB layer

**Files:**
- Create: `services/ml/pyproject.toml`
- Create: `services/ml/config.py`
- Create: `services/ml/db.py`

- [ ] **Step 1: Create pyproject.toml**

```toml
# services/ml/pyproject.toml
[project]
name = "briefing-ml"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
  "psycopg[binary]>=3.1",
  "umap-learn>=0.5",
  "hdbscan>=0.8",
  "numpy>=1.26",
  "requests>=2.31",
  "python-dotenv>=1.0",
]

[project.optional-dependencies]
test = ["pytest>=8.0"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"
```

- [ ] **Step 2: Install dependencies**

```bash
cd services/ml && pip install -e ".[test]"
```

- [ ] **Step 3: Write config.py**

```python
# services/ml/config.py
import os
from dotenv import load_dotenv

load_dotenv()

DATABASE_URL = os.environ["DATABASE_URL"]
TOGETHER_API_KEY = os.environ["TOGETHER_API_KEY"]
```

- [ ] **Step 4: Write db.py**

```python
# services/ml/db.py
import psycopg
from config import DATABASE_URL


def connect():
    return psycopg.connect(DATABASE_URL)


def fetch_unprocessed_items(conn, date: str) -> list[dict]:
    """Return processed_items without cluster_id ingested on the given date."""
    with conn.cursor() as cur:
        cur.execute("""
            SELECT pi.id, pi.embedding_text
            FROM processed_items pi
            JOIN ingested_items ii ON ii.id = pi.ingested_item_id
            WHERE pi.cluster_id IS NULL
              AND ii.ingested_at::date = %s
        """, (date,))
        return [{"id": str(row[0]), "embedding_text": row[1]} for row in cur.fetchall()]


def save_cluster(conn, briefing_date: str, label: str, item_count: int) -> str:
    with conn.cursor() as cur:
        cur.execute("""
            INSERT INTO clusters (briefing_date, label, item_count)
            VALUES (%s, %s, %s) RETURNING id
        """, (briefing_date, label, item_count))
        return str(cur.fetchone()[0])


def assign_cluster(conn, item_id: str, cluster_id: str):
    with conn.cursor() as cur:
        cur.execute(
            "UPDATE processed_items SET cluster_id = %s WHERE id = %s",
            (cluster_id, item_id),
        )
```

- [ ] **Step 5: Commit**

```bash
git add services/ml/pyproject.toml services/ml/config.py services/ml/db.py
git commit -m "feat(ml): add Python project setup and DB layer"
```

---

### Task 14: Embeddings + clustering

**Files:**
- Create: `services/ml/embeddings.py`
- Create: `services/ml/clustering.py`
- Create: `services/ml/tests/test_clustering.py`

- [ ] **Step 1: Write embeddings.py**

```python
# services/ml/embeddings.py
import requests
from config import TOGETHER_API_KEY

EMBEDDING_MODEL = "togethercomputer/m2-bert-80M-8k-retrieval"


def embed_texts(texts: list[str]) -> list[list[float]]:
    """Call TogetherAI Embeddings API and return vectors."""
    resp = requests.post(
        "https://api.together.xyz/v1/embeddings",
        headers={"Authorization": f"Bearer {TOGETHER_API_KEY}"},
        json={"model": EMBEDDING_MODEL, "input": texts},
        timeout=60,
    )
    resp.raise_for_status()
    data = resp.json()["data"]
    return [d["embedding"] for d in sorted(data, key=lambda x: x["index"])]
```

- [ ] **Step 2: Write the failing clustering test**

```python
# services/ml/tests/test_clustering.py
import numpy as np
from clustering import cluster_embeddings


def test_cluster_embeddings_groups_similar():
    """Items with identical vectors should land in the same cluster."""
    group_a = np.array([1.0, 0.0, 0.0, 0.0])
    group_b = np.array([0.0, 1.0, 0.0, 0.0])

    vectors = [
        group_a + np.random.normal(0, 0.01, 4),
        group_a + np.random.normal(0, 0.01, 4),
        group_a + np.random.normal(0, 0.01, 4),
        group_b + np.random.normal(0, 0.01, 4),
        group_b + np.random.normal(0, 0.01, 4),
        group_b + np.random.normal(0, 0.01, 4),
    ]

    labels = cluster_embeddings(vectors)
    assert len(labels) == 6
    # first 3 should share a cluster label
    assert labels[0] == labels[1] == labels[2]
    # last 3 should share a different cluster label
    assert labels[3] == labels[4] == labels[5]
    assert labels[0] != labels[3]
```

- [ ] **Step 3: Run test — expect FAIL**

```bash
cd services/ml && python -m pytest tests/test_clustering.py -v
```
Expected: `FAIL` — `ModuleNotFoundError: No module named 'clustering'`.

- [ ] **Step 4: Implement clustering.py**

```python
# services/ml/clustering.py
import numpy as np
import umap
import hdbscan


def cluster_embeddings(vectors: list[list[float]]) -> list[int]:
    """
    Reduce dimensionality with UMAP then cluster with HDBSCAN.
    Returns a list of integer cluster labels (same length as input).
    Label -1 means noise / unclustered.
    """
    arr = np.array(vectors, dtype=np.float32)

    n = len(arr)
    n_neighbors = min(5, n - 1)
    n_components = min(5, n - 1)

    reducer = umap.UMAP(
        n_neighbors=n_neighbors,
        n_components=n_components,
        metric="cosine",
        random_state=42,
    )
    reduced = reducer.fit_transform(arr)

    clusterer = hdbscan.HDBSCAN(min_cluster_size=2, metric="euclidean")
    labels = clusterer.fit_predict(reduced)
    return labels.tolist()
```

- [ ] **Step 5: Run test — expect PASS**

```bash
cd services/ml && python -m pytest tests/test_clustering.py -v
```
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add services/ml/embeddings.py services/ml/clustering.py services/ml/tests/
git commit -m "feat(ml): add TogetherAI embeddings and UMAP+HDBSCAN clustering"
```

---

### Task 15: Cluster labeler + pipeline entrypoint

**Files:**
- Create: `services/ml/labeler.py`
- Create: `services/ml/pipeline.py`
- Create: `services/ml/main.py`

- [ ] **Step 1: Write labeler.py**

```python
# services/ml/labeler.py
import requests
from config import TOGETHER_API_KEY

LLM_MODEL = "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo"


def label_cluster(embedding_texts: list[str]) -> str:
    """Ask the LLM to generate a short topic label for a cluster."""
    sample = "\n".join(f"- {t}" for t in embedding_texts[:10])
    prompt = (
        "The following are brief descriptions of articles grouped together by topic similarity.\n"
        f"{sample}\n\n"
        "Give this cluster a concise topic label (5 words or fewer). "
        "Return only the label, no punctuation."
    )
    resp = requests.post(
        "https://api.together.xyz/v1/chat/completions",
        headers={"Authorization": f"Bearer {TOGETHER_API_KEY}"},
        json={
            "model": LLM_MODEL,
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": 20,
        },
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()["choices"][0]["message"]["content"].strip()
```

- [ ] **Step 2: Write pipeline.py**

```python
# services/ml/pipeline.py
import logging
from collections import defaultdict

from clustering import cluster_embeddings
from db import fetch_unprocessed_items, save_cluster, assign_cluster
from embeddings import embed_texts
from labeler import label_cluster

log = logging.getLogger(__name__)


def run(conn, date: str):
    items = fetch_unprocessed_items(conn, date)
    if not items:
        log.info("no unprocessed items for %s", date)
        return

    log.info("clustering %d items for %s", len(items), date)

    texts = [item["embedding_text"] for item in items]
    vectors = embed_texts(texts)
    labels = cluster_embeddings(vectors)

    # Group item IDs by cluster label (skip noise label -1)
    clusters: dict[int, list[int]] = defaultdict(list)
    for idx, label in enumerate(labels):
        if label != -1:
            clusters[label].append(idx)

    log.info("found %d clusters (%d noise items)", len(clusters), labels.count(-1))

    for cluster_label, indices in clusters.items():
        cluster_texts = [texts[i] for i in indices]
        topic_label = label_cluster(cluster_texts)
        cluster_id = save_cluster(conn, date, topic_label, len(indices))

        for idx in indices:
            assign_cluster(conn, items[idx]["id"], cluster_id)

        log.info("saved cluster '%s' with %d items", topic_label, len(indices))

    conn.commit()
```

- [ ] **Step 3: Write main.py**

```python
# services/ml/main.py
import logging
import sys
from datetime import date

from db import connect
from pipeline import run

logging.basicConfig(
    level=logging.INFO,
    format='{"time":"%(asctime)s","level":"%(levelname)s","msg":"%(message)s"}',
)

if __name__ == "__main__":
    today = date.today().isoformat()
    target_date = sys.argv[1] if len(sys.argv) > 1 else today

    conn = connect()
    try:
        run(conn, target_date)
    finally:
        conn.close()
```

- [ ] **Step 4: Run pipeline against local DB with test data**

```bash
cd services/ml && DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  TOGETHER_API_KEY=your_key \
  python main.py
```
Expected: log line `no unprocessed items for <date>` (since no items yet) or successful clustering if items exist.

- [ ] **Step 5: Commit**

```bash
git add services/ml/labeler.py services/ml/pipeline.py services/ml/main.py
git commit -m "feat(ml): add cluster labeler and pipeline orchestration"
```

---

## Group 5: API Service

### Task 16: API DB layer

**Files:**
- Create: `services/api/config/config.go`
- Create: `services/api/db/db.go`
- Create: `services/api/db/queries.go`
- Create: `services/api/db/queries_test.go`

- [ ] **Step 1: Add pgx dependency**

```bash
cd services/api && go get github.com/jackc/pgx/v5
```

- [ ] **Step 2: Write config.go**

```go
// services/api/config/config.go
package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL    string
	TogetherAPIKey string
	Port           string
}

func Load() Config {
	dbURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("TOGETHER_API_KEY")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if dbURL == "" || apiKey == "" {
		slog.Error("DATABASE_URL and TOGETHER_API_KEY are required")
		os.Exit(1)
	}
	return Config{DatabaseURL: dbURL, TogetherAPIKey: apiKey, Port: port}
}
```

- [ ] **Step 3: Write db.go**

```go
// services/api/db/db.go
package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
```

- [ ] **Step 4: Write queries.go**

```go
// services/api/db/queries.go
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
```

- [ ] **Step 5: Write queries_test.go (integration)**

```go
// services/api/db/queries_test.go
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

	source, err := db.AddSource(ctx, pool, "rss", "https://test.example.com/feed", "Test Source")
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
```

- [ ] **Step 6: Run integration test**

```bash
cd services/api && DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  go test ./db/... -v
```
Expected: `PASS`.

- [ ] **Step 7: Commit**

```bash
git add services/api/config/ services/api/db/
git commit -m "feat(api): add config and DB layer with source/briefing/item queries"
```

---

### Task 17: TogetherAI client + briefing generator

**Files:**
- Create: `services/api/ai/together.go`
- Create: `services/api/briefing/generator.go`
- Create: `services/api/briefing/generator_test.go`

- [ ] **Step 1: Copy the TogetherAI client pattern (same implementation as processing)**

```go
// services/api/ai/together.go
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.together.xyz/v1"

type TogetherClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewTogetherClient(apiKey, baseURL string) *TogetherClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &TogetherClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *TogetherClient) Complete(prompt string) (string, error) {
	body := chatRequest{
		Model:     "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo",
		Messages:  []chatMessage{{Role: "user", Content: prompt}},
		MaxTokens: 1024,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("together API returned %d", resp.StatusCode)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}
```

- [ ] **Step 2: Write the failing generator test**

```go
// services/api/briefing/generator_test.go
package briefing_test

import (
	"strings"
	"testing"

	"api/briefing"
	"api/db"
)

type mockAI struct{}

func (m *mockAI) Complete(prompt string) (string, error) {
	return "Mock analysis paragraph.", nil
}

func TestGenerateBriefing(t *testing.T) {
	clusters := []db.Cluster{
		{
			ID:    "c1",
			Label: "Go Programming",
			Items: []db.ClusterItem{
				{Title: "Go 1.23 Released", URL: "https://example.com/go123", Summary: "New Go version."},
				{Title: "Go Performance Tips", URL: "https://example.com/perf", Summary: "Make Go faster."},
			},
		},
		{
			ID:    "c2",
			Label: "AI News",
			Items: []db.ClusterItem{
				{Title: "LLM Advances", URL: "https://example.com/llm", Summary: "New models."},
			},
		},
	}

	g := briefing.NewGenerator(&mockAI{})
	content, err := g.Generate(clusters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "Go Programming") {
		t.Error("expected cluster label in output")
	}
	if !strings.Contains(content, "AI News") {
		t.Error("expected second cluster label in output")
	}
	if len(content) < 50 {
		t.Error("briefing content too short")
	}
}
```

- [ ] **Step 3: Run test — expect FAIL**

```bash
cd services/api && go test ./briefing/... -v
```
Expected: `FAIL` — `briefing.NewGenerator undefined`.

- [ ] **Step 4: Implement generator.go**

```go
// services/api/briefing/generator.go
package briefing

import (
	"fmt"
	"strings"

	"api/db"
)

type Completer interface {
	Complete(prompt string) (string, error)
}

type Generator struct {
	ai Completer
}

func NewGenerator(ai Completer) *Generator {
	return &Generator{ai: ai}
}

func (g *Generator) Generate(clusters []db.Cluster) (string, error) {
	var sections []string
	for _, c := range clusters {
		analysis, err := g.ai.Complete(clusterPrompt(c))
		if err != nil {
			return "", fmt.Errorf("cluster %q analysis: %w", c.Label, err)
		}
		sections = append(sections, fmt.Sprintf("## %s\n\n%s", c.Label, analysis))
	}

	combined := strings.Join(sections, "\n\n---\n\n")
	final, err := g.ai.Complete(synthesisPrompt(combined))
	if err != nil {
		return "", fmt.Errorf("final synthesis: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Daily Briefing\n\n")
	sb.WriteString(final)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString(combined)
	return sb.String(), nil
}

func clusterPrompt(c db.Cluster) string {
	var items strings.Builder
	for _, item := range c.Items {
		items.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", item.Title, item.URL, item.Summary))
	}
	return fmt.Sprintf(
		"You are writing a section of a personal daily briefing.\n"+
			"Topic: %s\n\nArticles:\n%s\n\n"+
			"Write a concise 2-3 paragraph analysis of what's happening in this topic area. "+
			"Be direct and insightful. No preamble.",
		c.Label, items.String(),
	)
}

func synthesisPrompt(sections string) string {
	return fmt.Sprintf(
		"You are writing the executive summary of a personal daily briefing.\n"+
			"Below are topic-by-topic analyses:\n\n%s\n\n"+
			"Write a 3-4 sentence overview that ties the major themes together. "+
			"Be concise. No preamble.",
		sections,
	)
}
```

- [ ] **Step 5: Run test — expect PASS**

```bash
cd services/api && go test ./briefing/... -v
```
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add services/api/ai/ services/api/briefing/
git commit -m "feat(api): add TogetherAI client and briefing generator"
```

---

### Task 18: templ templates

**Files:**
- Create: `services/api/templates/layout.templ`
- Create: `services/api/templates/briefing.templ`
- Create: `services/api/templates/sources.templ`
- Create: `services/api/templates/items.templ`
- Create: `services/api/templates/fragments/source_row.templ`
- Create: `services/api/templates/fragments/items_list.templ`

- [ ] **Step 1: Install templ CLI and add dependency**

```bash
go install github.com/a-h/templ/cmd/templ@latest
cd services/api && go get github.com/a-h/templ
go get github.com/yuin/goldmark
```

- [ ] **Step 2: Write layout.templ**

```go
// services/api/templates/layout.templ
package templates

templ Layout(title string) {
  <!DOCTYPE html>
  <html lang="en">
  <head>
    <meta charset="UTF-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    <title>{ title } — Curator</title>
    <script src="https://unpkg.com/htmx.org@1.9.12"></script>
    <style>
      body { font-family: system-ui, sans-serif; max-width: 860px; margin: 2rem auto; padding: 0 1rem; line-height: 1.6; }
      nav a { margin-right: 1rem; text-decoration: none; color: #555; }
      nav a:hover { color: #000; }
      .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: .8rem; }
      .badge-pending { background: #fff3cd; color: #856404; }
      .badge-done { background: #d1e7dd; color: #0a3622; }
      .badge-error { background: #f8d7da; color: #842029; }
      table { width: 100%; border-collapse: collapse; }
      th, td { padding: .5rem; text-align: left; border-bottom: 1px solid #eee; }
      button { cursor: pointer; }
    </style>
  </head>
  <body>
    <nav>
      <a href="/">Briefing</a>
      <a href="/sources">Sources</a>
      <a href="/items">Items</a>
    </nav>
    <hr/>
    { children... }
  </body>
  </html>
}
```

- [ ] **Step 3: Write briefing.templ**

```go
// services/api/templates/briefing.templ
package templates

import "html/template"

templ BriefingPage(date string, contentHTML template.HTML) {
  @Layout("Briefing " + date) {
    <h1>Briefing — { date }</h1>
    <div class="briefing-content">
      @templ.Raw(string(contentHTML))
    </div>
  }
}

templ NoBriefingPage() {
  @Layout("No Briefing Yet") {
    <h1>No briefing yet</h1>
    <p>The pipeline hasn't generated a briefing for today. Check back later.</p>
  }
}
```

- [ ] **Step 4: Write sources.templ**

```go
// services/api/templates/sources.templ
package templates

import (
  "api/db"
  "api/templates/fragments"
)

templ SourcesPage(sources []db.Source) {
  @Layout("Sources") {
    <h1>Sources</h1>
    <form hx-post="/sources" hx-target="#source-list" hx-swap="outerHTML">
      <select name="type" required>
        <option value="rss">RSS / Substack</option>
        <option value="reddit">Reddit</option>
        <option value="hn">Hacker News</option>
      </select>
      <input type="text" name="name" placeholder="Name" required/>
      <input type="url" name="url" placeholder="URL" required/>
      <button type="submit">Add Source</button>
    </form>
    <br/>
    @SourceList(sources)
  }
}

templ SourceList(sources []db.Source) {
  <table id="source-list">
    <thead>
      <tr><th>Name</th><th>Type</th><th>URL</th><th>Active</th></tr>
    </thead>
    <tbody>
      for _, s := range sources {
        @fragments.SourceRow(s)
      }
    </tbody>
  </table>
}
```

- [ ] **Step 5: Write fragments/source_row.templ**

```go
// services/api/templates/fragments/source_row.templ
package fragments

import "api/db"

templ SourceRow(s db.Source) {
  <tr id={ "source-" + s.ID }>
    <td>{ s.Name }</td>
    <td>{ s.Type }</td>
    <td><a href={ templ.URL(s.URL) } target="_blank">{ s.URL }</a></td>
    <td>
      if s.Active {
        <button hx-patch={ "/sources/" + s.ID + "/toggle" }
                hx-target={ "#source-" + s.ID }
                hx-swap="outerHTML">Disable</button>
      } else {
        <button hx-patch={ "/sources/" + s.ID + "/toggle" }
                hx-target={ "#source-" + s.ID }
                hx-swap="outerHTML">Enable</button>
      }
    </td>
  </tr>
}
```

- [ ] **Step 6: Write items.templ and fragments/items_list.templ**

```go
// services/api/templates/items.templ
package templates

import "api/db"

templ ItemsPage(items []db.Item, sources []db.Source) {
  @Layout("Items") {
    <h1>Ingested Items</h1>
    <form hx-get="/items" hx-target="#items-list" hx-swap="outerHTML" hx-trigger="change">
      <select name="source">
        <option value="">All sources</option>
        for _, s := range sources {
          <option value={ s.ID }>{ s.Name }</option>
        }
      </select>
      <select name="status">
        <option value="">All statuses</option>
        <option value="pending">Pending</option>
        <option value="processing">Processing</option>
        <option value="done">Done</option>
        <option value="error">Error</option>
      </select>
    </form>
    <br/>
    @ItemsList(items)
  }
}
```

```go
// services/api/templates/fragments/items_list.templ
package fragments

import (
  "api/db"
  "fmt"
)

templ ItemsList(items []db.Item) {
  <table id="items-list">
    <thead>
      <tr><th>Title</th><th>Source</th><th>Status</th><th>Ingested</th></tr>
    </thead>
    <tbody>
      for _, item := range items {
        <tr>
          <td><a href={ templ.URL(item.URL) } target="_blank">{ item.Title }</a></td>
          <td>{ item.SourceName }</td>
          <td><span class={ "badge badge-" + item.Status }>{ item.Status }</span></td>
          <td>{ fmt.Sprintf("%s", item.IngestedAt.Format("Jan 2 15:04")) }</td>
        </tr>
      }
    </tbody>
  </table>
}
```

- [ ] **Step 7: Generate Go code from templ files**

```bash
cd services/api && templ generate
```
Expected: generates `*_templ.go` files alongside each `.templ` file, no errors.

- [ ] **Step 8: Commit**

```bash
git add services/api/templates/ services/api/ai/
git commit -m "feat(api): add templ templates — layout, briefing, sources, items, fragments"
```

---

### Task 19: HTTP handlers

**Files:**
- Create: `services/api/handlers/briefings.go`
- Create: `services/api/handlers/sources.go`
- Create: `services/api/handlers/items.go`

- [ ] **Step 1: Add chi and goldmark**

```bash
cd services/api && go get github.com/go-chi/chi/v5 github.com/yuin/goldmark
```

- [ ] **Step 2: Write handlers/briefings.go**

```go
// services/api/handlers/briefings.go
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
```

- [ ] **Step 3: Write handlers/sources.go**

```go
// services/api/handlers/sources.go
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"api/db"
	"api/templates"
	"api/templates/fragments"
)

type SourceHandler struct {
	pool *pgxpool.Pool
}

func NewSourceHandler(pool *pgxpool.Pool) *SourceHandler {
	return &SourceHandler{pool: pool}
}

func (h *SourceHandler) List(w http.ResponseWriter, r *http.Request) {
	sources, err := db.ListSources(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	templates.SourcesPage(sources).Render(r.Context(), w)
}

func (h *SourceHandler) Add(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sourceType := r.FormValue("type")
	name := r.FormValue("name")
	url := r.FormValue("url")

	if sourceType == "" || name == "" || url == "" {
		http.Error(w, "type, name, and url are required", http.StatusBadRequest)
		return
	}

	sources, err := db.ListSources(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if _, err := db.AddSource(r.Context(), h.pool, sourceType, url, name); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	sources, _ = db.ListSources(r.Context(), h.pool)
	templates.SourceList(sources).Render(r.Context(), w)
}

func (h *SourceHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	source, err := db.ToggleSource(r.Context(), h.pool, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	fragments.SourceRow(source).Render(r.Context(), w)
}
```

- [ ] **Step 4: Write handlers/items.go**

```go
// services/api/handlers/items.go
package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"api/db"
	"api/templates"
	"api/templates/fragments"
)

type ItemHandler struct {
	pool *pgxpool.Pool
}

func NewItemHandler(pool *pgxpool.Pool) *ItemHandler {
	return &ItemHandler{pool: pool}
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("source")
	status := r.URL.Query().Get("status")

	items, err := db.ListItems(r.Context(), h.pool, sourceID, status)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	sources, _ := db.ListSources(r.Context(), h.pool)

	// htmx requests get only the fragment; full page requests get the full layout
	if r.Header.Get("HX-Request") == "true" {
		fragments.ItemsList(items).Render(r.Context(), w)
		return
	}
	templates.ItemsPage(items, sources).Render(r.Context(), w)
}
```

- [ ] **Step 5: Build to verify compilation**

```bash
cd services/api && go build ./...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add services/api/handlers/
git commit -m "feat(api): add HTTP handlers for briefings, sources, and items"
```

---

### Task 20: API main entrypoint + brief-gen cron

**Files:**
- Create: `services/api/main.go`

- [ ] **Step 1: Write main.go**

```go
// services/api/main.go
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

	// Brief generation runs in background on schedule
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
		// Run brief generation at :30 past 1, 7, 13, 19
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
	// Tomorrow at 01:30
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
```

- [ ] **Step 2: Fix missing import (pgxpool in main.go)**

Add `"github.com/jackc/pgx/v5/pgxpool"` to imports and change the `runBriefingCron` signature:

```go
func runBriefingCron(ctx context.Context, pool *pgxpool.Pool, gen *briefing.Generator) {
```

- [ ] **Step 3: Build final binary**

```bash
cd services/api && go build ./...
```
Expected: no errors.

- [ ] **Step 4: Run API server locally**

```bash
DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  TOGETHER_API_KEY=dummy \
  PORT=8080 \
  go run ./services/api/main.go
```
Expected: `{"level":"INFO","msg":"server starting","addr":":8080"}` — open `http://localhost:8080` in browser, should show "No briefing yet" page.

- [ ] **Step 5: Test source management UI**

Navigate to `http://localhost:8080/sources`. Add a source:
- Type: `hn`
- Name: `Hacker News`
- URL: `topstories`

Expected: source appears in table without page reload (htmx working).

- [ ] **Step 6: Commit**

```bash
git add services/api/main.go
git commit -m "feat(api): add main HTTP server and briefing generation cron"
```

---

## Group 6: End-to-End Smoke Test

### Task 21: Full pipeline smoke test

- [ ] **Step 1: Run migrations on local Postgres**

```bash
make migrate
```

- [ ] **Step 2: Seed HN source**

```bash
docker compose exec postgres psql -U curator -c \
  "INSERT INTO data_sources (type, url, name) VALUES ('hn', 'topstories', 'Hacker News Top') ON CONFLICT DO NOTHING"
```

- [ ] **Step 3: Run ingestion**

```bash
DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  go run ./services/ingestion/main.go
```
Expected: JSON logs with `"inserted"` count > 0.

- [ ] **Step 4: Verify items in DB**

```bash
docker compose exec postgres psql -U curator -c \
  "SELECT count(*), status FROM ingested_items GROUP BY status"
```
Expected: rows with `pending` status.

- [ ] **Step 5: Run processing (requires real TOGETHER_API_KEY)**

```bash
DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  TOGETHER_API_KEY=your_real_key \
  go run ./services/processing/main.go
```
Expected: JSON logs with `"item processed"` entries.

- [ ] **Step 6: Run ML service**

```bash
cd services/ml && \
  DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  TOGETHER_API_KEY=your_real_key \
  python main.py
```
Expected: logs showing clusters found and saved.

- [ ] **Step 7: Verify clusters in DB**

```bash
docker compose exec postgres psql -U curator -c \
  "SELECT label, item_count FROM clusters"
```
Expected: rows with topic labels.

- [ ] **Step 8: Start API and trigger briefing generation manually**

```bash
DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
  TOGETHER_API_KEY=your_real_key \
  PORT=8080 \
  go run ./services/api/main.go &
```
Then trigger brief generation by temporarily calling `generateBriefing` directly, or wait for the cron slot.

For manual trigger, add a temporary `POST /admin/generate` route to `main.go` for testing:
```bash
curl -X POST http://localhost:8080/admin/generate
```

- [ ] **Step 9: View briefing in browser**

Open `http://localhost:8080` — should redirect to today's briefing with rendered Markdown content.

- [ ] **Step 10: Final commit**

```bash
git add .
git commit -m "chore: end-to-end smoke test verified — full pipeline working"
```

---

## Railway Deployment Checklist

After the smoke test passes:

- [ ] Create Railway project, add PostgreSQL service
- [ ] Deploy `services/ingestion` as a cron service: `0 */6 * * *`
- [ ] Deploy `services/processing` as a cron service: `30 */6 * * *`
- [ ] Deploy `services/ml` as a cron service: `0 1,7,13,19 * * *`
- [ ] Deploy `services/api` as a web service (always-on)
- [ ] Set `DATABASE_URL` and `TOGETHER_API_KEY` as Railway shared env vars
- [ ] Run `make migrate` against Railway Postgres URL
- [ ] Seed initial data sources via the `/sources` UI
