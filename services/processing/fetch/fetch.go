package fetch

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var client = &http.Client{Timeout: 15 * time.Second}

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
