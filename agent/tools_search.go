package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentplexus/omniserp"
	"github.com/agentplexus/omniserp/client"
)

// SearchTool provides web search capabilities via omniserp.
type SearchTool struct {
	client *client.Client
}

// SearchArgs are the arguments for the search tool.
type SearchArgs struct {
	Query string `json:"query"`
	Type  string `json:"type,omitempty"` // "web", "news", "images" (default: "web")
}

// NewSearchTool creates a new search tool.
func NewSearchTool() (*SearchTool, error) {
	c, err := client.NewWithOptions(&client.Options{
		Silent: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create search client: %w", err)
	}

	return &SearchTool{client: c}, nil
}

func (t *SearchTool) Name() string {
	return "web_search"
}

func (t *SearchTool) Description() string {
	return "Search the web for current information. Use this when you need up-to-date information, news, or facts that may not be in your training data."
}

func (t *SearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search query",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"web", "news", "images"},
				"description": "Type of search (default: web)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchTool) Execute(ctx context.Context, argsJSON json.RawMessage) (string, error) {
	var args SearchArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	params := omniserp.SearchParams{
		Query: args.Query,
	}

	var result *omniserp.NormalizedSearchResult
	var err error

	switch args.Type {
	case "news":
		result, err = t.client.SearchNewsNormalized(ctx, params)
	case "images":
		result, err = t.client.SearchImagesNormalized(ctx, params)
	default:
		result, err = t.client.SearchNormalized(ctx, params)
	}

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	return formatSearchResults(result), nil
}

// formatSearchResults converts search results to a readable string.
func formatSearchResults(result *omniserp.NormalizedSearchResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Search results for: %s\n\n", result.SearchMetadata.Query))

	// Answer box if available (show first)
	if result.AnswerBox != nil && result.AnswerBox.Answer != "" {
		sb.WriteString("Direct Answer:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", result.AnswerBox.Answer))
		sb.WriteString("\n")
	}

	// Knowledge graph if available
	if result.KnowledgeGraph != nil && result.KnowledgeGraph.Title != "" {
		sb.WriteString("Knowledge Panel:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", result.KnowledgeGraph.Title))
		if result.KnowledgeGraph.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", result.KnowledgeGraph.Description))
		}
		sb.WriteString("\n")
	}

	// Organic results
	for i, item := range result.OrganicResults {
		if i >= 5 {
			break // Limit to top 5 results
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", item.Link))
		if item.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", item.Snippet))
		}
		sb.WriteString("\n")
	}

	// News results if available
	for i, item := range result.NewsResults {
		if i >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("News: %s\n", item.Title))
		sb.WriteString(fmt.Sprintf("   Source: %s | %s\n", item.Source, item.Date))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", item.Link))
		sb.WriteString("\n")
	}

	return sb.String()
}

// Ensure SearchTool implements Tool interface.
var _ Tool = (*SearchTool)(nil)
