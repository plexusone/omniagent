// Package browser provides browser automation tools for omniagent.
package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/agentplexus/omniagent/agent"
)

// Tool provides browser automation capabilities.
type Tool struct {
	browser  *rod.Browser
	page     *rod.Page
	headless bool
	logger   *slog.Logger
}

// Config configures the browser tool.
type Config struct {
	Headless bool
	UserData string
	Logger   *slog.Logger
}

// New creates a new browser tool.
func New(config Config) (*Tool, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Tool{
		headless: config.Headless,
		logger:   config.Logger,
	}, nil
}

// Name returns the tool name.
func (t *Tool) Name() string {
	return "browser"
}

// Description returns the tool description.
func (t *Tool) Description() string {
	return "Control a web browser to navigate pages, click elements, fill forms, and take screenshots."
}

// Parameters returns the JSON schema for tool parameters.
func (t *Tool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The browser action to perform",
				"enum":        []string{"navigate", "click", "type", "screenshot", "get_text", "wait"},
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to navigate to (for navigate action)",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector for the element (for click, type, get_text actions)",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to type (for type action)",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
			},
		},
		"required": []string{"action"},
	}
}

// Execute runs the browser tool.
func (t *Tool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action   string `json:"action"`
		URL      string `json:"url"`
		Selector string `json:"selector"`
		Text     string `json:"text"`
		Timeout  int    `json:"timeout"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse parameters: %w", err)
	}

	if params.Timeout == 0 {
		params.Timeout = 30
	}

	// Ensure browser is launched
	if err := t.ensureBrowser(); err != nil {
		return "", err
	}

	timeout := time.Duration(params.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch params.Action {
	case "navigate":
		return t.navigate(ctx, params.URL)
	case "click":
		return t.click(ctx, params.Selector)
	case "type":
		return t.typeText(ctx, params.Selector, params.Text)
	case "screenshot":
		return t.screenshot(ctx)
	case "get_text":
		return t.getText(ctx, params.Selector)
	case "wait":
		return t.wait(ctx, params.Selector)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// ensureBrowser ensures the browser is launched.
func (t *Tool) ensureBrowser() error {
	if t.browser != nil {
		return nil
	}

	l := launcher.New().Headless(t.headless)
	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}

	t.browser = rod.New().ControlURL(url)
	if err := t.browser.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}

	t.page, err = t.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	t.logger.Info("browser launched", "headless", t.headless)
	return nil
}

// navigate navigates to a URL.
func (t *Tool) navigate(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url required for navigate action")
	}

	if err := t.page.Context(ctx).Navigate(url); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}

	if err := t.page.WaitStable(time.Second); err != nil {
		return "", fmt.Errorf("wait stable: %w", err)
	}

	title := t.page.MustInfo().Title

	return fmt.Sprintf("Navigated to: %s (title: %s)", url, title), nil
}

// click clicks an element.
func (t *Tool) click(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for click action")
	}

	el, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element: %w", err)
	}

	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("click: %w", err)
	}

	return fmt.Sprintf("Clicked element: %s", selector), nil
}

// typeText types text into an element.
func (t *Tool) typeText(ctx context.Context, selector, text string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for type action")
	}

	el, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element: %w", err)
	}

	if err := el.Input(text); err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	return fmt.Sprintf("Typed text into: %s", selector), nil
}

// screenshot takes a screenshot.
func (t *Tool) screenshot(ctx context.Context) (string, error) {
	data, err := t.page.Context(ctx).Screenshot(false, nil)
	if err != nil {
		return "", fmt.Errorf("screenshot: %w", err)
	}

	// In a real implementation, you might save this or return the base64 data
	_ = base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("Screenshot taken (%d bytes)", len(data)), nil
}

// getText gets text from an element.
func (t *Tool) getText(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for get_text action")
	}

	el, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element: %w", err)
	}

	text, err := el.Text()
	if err != nil {
		return "", fmt.Errorf("get text: %w", err)
	}

	return text, nil
}

// wait waits for an element to appear.
func (t *Tool) wait(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for wait action")
	}

	_, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("wait for element: %w", err)
	}

	return fmt.Sprintf("Element found: %s", selector), nil
}

// Close closes the browser.
func (t *Tool) Close() error {
	if t.browser != nil {
		return t.browser.Close()
	}
	return nil
}

// Ensure Tool implements agent.Tool interface.
var _ agent.Tool = (*Tool)(nil)
