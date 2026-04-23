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
	if items[1].URL == "" {
		t.Error("URL should fall back to HN item URL when post URL is empty")
	}
}
