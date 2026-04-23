package processor

import "fmt"

type Completer interface {
	Complete(prompt string) (string, error)
}

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
