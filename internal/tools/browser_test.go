package tools

import (
	"context"
	"github.com/grok-agent/backend/internal/sandbox"
	"os"
	"testing"
)

func TestEvaluateRejectsEmptyScript(t *testing.T) {
	result, err := NewBrowserTool(nil, "").Execute(context.Background(), ToolInput{"action": "evaluate"})
	if err != nil || result.Error != "script is required" {
		t.Fatalf("invalid evaluate arguments accepted: %#v %v", result, err)
	}
}

// Opt-in integration check against a browser containing a generated blob image.
func TestDownloadGeneratedImage(t *testing.T) {
	id := os.Getenv("TEST_SANDBOX_ID")
	if id == "" {
		t.Skip("set TEST_SANDBOX_ID to a computer with a generated image")
	}
	mgr, err := sandbox.NewManager("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	result, err := NewBrowserTool(mgr, id).Execute(context.Background(), ToolInput{"action": "download", "selector": "img[src^='blob:']", "path": "generated/gemini-image.jpg"})
	if err != nil || result.Error != "" {
		t.Fatalf("download failed: %#v %v", result, err)
	}
	t.Log(result.Output)
}

func TestDownloadInvalidDestination(t *testing.T) {
	for _, path := range []string{"../secret", ".browser/Cookies", "/etc/passwd"} {
		result, err := NewBrowserTool(nil, "").Execute(context.Background(), ToolInput{"action": "download", "path": path, "url": "blob:test"})
		if err != nil || result.Error == "" {
			t.Fatalf("unsafe download destination accepted: %q", path)
		}
	}
}

func TestBrowserTransfersRejectInvalidPaths(t *testing.T) {
	for _, action := range []string{"upload", "download_click"} {
		for _, path := range []string{"../secret", ".browser/Cookies", "/etc/passwd"} {
			result, err := NewBrowserTool(nil, "").Execute(context.Background(), ToolInput{"action": action, "path": path, "selector": "input"})
			if err != nil || result.Error == "" {
				t.Fatalf("%s accepted unsafe path %s", action, path)
			}
		}
		result, err := NewBrowserTool(nil, "").Execute(context.Background(), ToolInput{"action": action, "path": "assets/test.mp4"})
		if err != nil || result.Error != "selector required" {
			t.Fatal("missing selector accepted")
		}
	}
}

// Opt-in check against a disposable browser computer, never a user's active page.
func TestNativeBrowserTransfers(t *testing.T) {
	id := os.Getenv("TEST_TRANSFER_SANDBOX_ID")
	if id == "" {
		t.Skip("set TEST_TRANSFER_SANDBOX_ID to a disposable computer")
	}
	mgr, err := sandbox.NewManager("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	setup := `
from playwright.sync_api import sync_playwright
import os
os.makedirs('/workspace/transfer-check',exist_ok=True)
open('/workspace/transfer-check/input.txt','w').write('transfer fixture')
with sync_playwright() as p:
 b=p.chromium.connect_over_cdp('http://127.0.0.1:9222')
 page=b.contexts[0].pages[0]
 page.set_content('<input type="file" id="upload"><a id="download" download="sample.txt" href="data:text/plain,export%20fixture">Download</a>')
`
	res, err := mgr.ExecPython(context.Background(), id, setup)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("setup: %+v %v", res, err)
	}
	tool := NewBrowserTool(mgr, id)
	for _, input := range []ToolInput{
		{"action": "upload", "selector": "#upload", "path": "transfer-check/input.txt"},
		{"action": "download_click", "selector": "#download", "path": "transfer-check/output.txt"},
	} {
		result, err := tool.Execute(context.Background(), input)
		if err != nil || result.Error != "" {
			t.Fatalf("transfer: %+v %v", result, err)
		}
	}
	res, err = mgr.ExecPython(context.Background(), id, `
from playwright.sync_api import sync_playwright
assert open('/workspace/transfer-check/output.txt').read()=='export fixture'
with sync_playwright() as p:
 b=p.chromium.connect_over_cdp('http://127.0.0.1:9222')
 assert b.contexts[0].pages[0].locator('#upload').evaluate('el=>el.files[0].name')=='input.txt'
print('upload and download content verified')
`)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("verify: %+v %v", res, err)
	}
}
