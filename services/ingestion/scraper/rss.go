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
