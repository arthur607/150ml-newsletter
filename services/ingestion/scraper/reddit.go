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
