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
