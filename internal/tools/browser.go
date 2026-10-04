package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grok-agent/backend/internal/sandbox"
)

// BrowserTool controls the Playwright browser in the sandbox.
type BrowserTool struct {
	manager     *sandbox.Manager
	containerID string
}

func NewBrowserTool(m *sandbox.Manager, containerID string) *BrowserTool {
	return &BrowserTool{manager: m, containerID: containerID}
}

func (t *BrowserTool) Name() string { return "browser" }
func (t *BrowserTool) Description() string {
	return "Control a real web browser in the sandbox. Actions: navigate, click, type, screenshot, get_text, scroll."
}
func (t *BrowserTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action":   {"type": "string", "enum": ["navigate", "click", "type", "screenshot", "get_text", "scroll", "evaluate"], "description": "Browser action"},
			"url":      {"type": "string", "description": "URL to navigate to (for navigate action)"},
			"selector": {"type": "string", "description": "CSS selector for target element"},
			"text":     {"type": "string", "description": "Text to type (for type action)"},
			"script":   {"type": "string", "description": "JavaScript to evaluate (for evaluate action)"},
			"direction": {"type": "string", "enum": ["up", "down"], "description": "Scroll direction"}
		},
		"required": ["action"]
	}`)
}
func (t *BrowserTool) Execute(ctx context.Context, input ToolInput) (*ToolResult, error) {
	action, _ := input["action"].(string)

	// Each action is implemented as a Playwright Python script run inside the sandbox
	var script string
	switch action {
	case "navigate":
		url, _ := input["url"].(string)
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0] if browser.contexts else browser.new_context().new_page()
    page.goto(%q, wait_until="domcontentloaded", timeout=30000)
    print("Navigated to:", page.url)
    print("Title:", page.title())
`, url)

	case "get_text":
		selector, _ := input["selector"].(string)
		if selector == "" {
			selector = "body"
		}
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0]
    el = page.query_selector(%q)
    print(el.inner_text() if el else "Element not found")
`, selector)

	case "click":
		selector, _ := input["selector"].(string)
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0]
    page.click(%q)
    print("Clicked:", %q)
`, selector, selector)

	case "type":
		selector, _ := input["selector"].(string)
		text, _ := input["text"].(string)
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0]
    page.fill(%q, %q)
    print("Typed into:", %q)
`, selector, text, selector)

	case "screenshot":
		script = `
from playwright.sync_api import sync_playwright
import base64
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0]
    data = page.screenshot()
    print(base64.b64encode(data).decode())
`

	case "evaluate":
		js, _ := input["script"].(string)
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0]
    result = page.evaluate(%q)
    print(result)
`, js)

	case "scroll":
		dir, _ := input["direction"].(string)
		delta := 500
		if dir == "up" {
			delta = -500
		}
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://localhost:9222")
    page = browser.contexts[0].pages[0]
    page.evaluate("window.scrollBy(0, %d)")
    print("Scrolled %s")
`, delta, dir)

	default:
		return &ToolResult{Error: fmt.Sprintf("unknown browser action: %s", action)}, nil
	}

	res, err := t.manager.ExecPython(ctx, t.containerID, script)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	out := res.Stdout
	if res.Stderr != "" && res.ExitCode != 0 {
		return &ToolResult{Error: res.Stderr}, nil
	}
	return &ToolResult{Output: out}, nil
}
