package briefing

import (
	"fmt"
	"strings"

	"api/db"
)

type Completer interface {
	Complete(prompt string) (string, error)
}

type Generator struct {
	ai Completer
}

func NewGenerator(ai Completer) *Generator {
	return &Generator{ai: ai}
}

func (g *Generator) Generate(clusters []db.Cluster) (string, error) {
	var sections []string
	for _, c := range clusters {
		analysis, err := g.ai.Complete(clusterPrompt(c))
		if err != nil {
			return "", fmt.Errorf("cluster %q analysis: %w", c.Label, err)
		}
		sections = append(sections, fmt.Sprintf("## %s\n\n%s", c.Label, analysis))
	}

	combined := strings.Join(sections, "\n\n---\n\n")
	final, err := g.ai.Complete(synthesisPrompt(combined))
	if err != nil {
		return "", fmt.Errorf("final synthesis: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Daily Briefing\n\n")
	sb.WriteString(final)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString(combined)
	return sb.String(), nil
}

func clusterPrompt(c db.Cluster) string {
	var items strings.Builder
	for _, item := range c.Items {
		items.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", item.Title, item.URL, item.Summary))
	}
	return fmt.Sprintf(
		"You are writing a section of a personal daily briefing.\n"+
			"Topic: %s\n\nArticles:\n%s\n\n"+
			"Write a concise 2-3 paragraph analysis of what's happening in this topic area. "+
			"Be direct and insightful. No preamble.",
		c.Label, items.String(),
	)
}

func synthesisPrompt(sections string) string {
	return fmt.Sprintf(
		"You are writing the executive summary of a personal daily briefing.\n"+
			"Below are topic-by-topic analyses:\n\n%s\n\n"+
			"Write a 3-4 sentence overview that ties the major themes together. "+
			"Be concise. No preamble.",
		sections,
	)
}
