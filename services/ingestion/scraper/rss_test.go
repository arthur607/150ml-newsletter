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
