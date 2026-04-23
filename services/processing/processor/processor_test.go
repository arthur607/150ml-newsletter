package processor_test

import (
	"testing"

	"processing/processor"
)

func TestBuildPrompts(t *testing.T) {
	content := "This is article content about Go programming."
	summaryPrompt := processor.SummaryPrompt(content)
	if len(summaryPrompt) < 20 {
		t.Error("summary prompt too short")
	}
	embeddingPrompt := processor.EmbeddingTextPrompt(content, "mock summary")
	if len(embeddingPrompt) < 20 {
		t.Error("embedding text prompt too short")
	}
}
