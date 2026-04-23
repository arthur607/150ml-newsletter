package scraper

import "time"

type Item struct {
	ExternalID  string
	Title       string
	URL         string
	Author      string
	PublishedAt *time.Time
	RawContent  string
}

type Scraper interface {
	Scrape(sourceURL string) ([]Item, error)
}
