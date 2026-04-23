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
