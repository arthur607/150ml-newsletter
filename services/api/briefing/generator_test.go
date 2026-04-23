package briefing_test

import (
	"strings"
	"testing"

	"api/briefing"
	"api/db"
)

type mockAI struct{}

func (m *mockAI) Complete(prompt string) (string, error) {
	return "Mock analysis paragraph.", nil
}

func TestGenerateBriefing(t *testing.T) {
	clusters := []db.Cluster{
		{
			ID:    "c1",
			Label: "Go Programming",
			Items: []db.ClusterItem{
				{Title: "Go 1.23 Released", URL: "https://example.com/go123", Summary: "New Go version."},
				{Title: "Go Performance Tips", URL: "https://example.com/perf", Summary: "Make Go faster."},
			},
		},
		{
			ID:    "c2",
			Label: "AI News",
			Items: []db.ClusterItem{
				{Title: "LLM Advances", URL: "https://example.com/llm", Summary: "New models."},
			},
		},
	}

	g := briefing.NewGenerator(&mockAI{})
	content, err := g.Generate(clusters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "Go Programming") {
		t.Error("expected cluster label in output")
	}
	if !strings.Contains(content, "AI News") {
		t.Error("expected second cluster label in output")
	}
	if len(content) < 50 {
		t.Error("briefing content too short")
	}
}
