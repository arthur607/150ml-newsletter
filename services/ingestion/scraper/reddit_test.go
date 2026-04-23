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
