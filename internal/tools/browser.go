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
	return "Control the visible browser shared with the user. Actions: navigate, click, type, screenshot, get_text, scroll, evaluate, download, download_click, upload. Use upload with an observed file-input selector and existing workspace file path. Use download_click to capture a native download into a workspace path. Use download with an image selector or URL and a workspace path to save generated images (including blob URLs) directly. Files saved in /workspace appear as downloadable attachments in chat. Never print binary or base64 data; save files instead."
}
func (t *BrowserTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action":   {"type": "string", "enum": ["navigate", "click", "type", "screenshot", "get_text", "scroll", "evaluate", "download", "download_click", "upload"], "description": "Browser action"},
			"path":     {"type": "string", "description": "Workspace file path: destination for downloads, existing source for upload"},
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
	case "upload", "download_click":
		name, _ := input["path"].(string)
		target, err := sandbox.WorkspacePath(name)
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		selector, _ := input["selector"].(string)
		if selector == "" {
			return &ToolResult{Error: "selector required"}, nil
		}
		args, _ := json.Marshal(map[string]string{"path": target, "selector": selector, "action": action})
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
import json,os
args=json.loads(%q)
target=os.path.realpath(args['path'])
assert target.startswith('/workspace/') and not any(p.startswith('.') for p in os.path.relpath(target,'/workspace').split('/')), 'invalid workspace path'
with sync_playwright() as p:
 browser=p.chromium.connect_over_cdp('http://127.0.0.1:9222',timeout=15000)
 page=browser.contexts[0].pages[0]
 if args['action']=='upload':
  assert os.path.isfile(target), 'upload file not found'
  page.locator(args['selector']).first.set_input_files(target,timeout=30000)
  print(json.dumps({'uploaded':os.path.relpath(target,'/workspace'),'message':'File selected; inspect site to verify upload completion.'}))
 else:
  assert not os.path.exists(target), 'destination already exists; choose a new filename'
  os.makedirs(os.path.dirname(target),exist_ok=True)
  with page.expect_download(timeout=60000) as event:
   page.locator(args['selector']).first.click(timeout=15000)
  download=event.value
  download.save_as(target)
  assert os.path.getsize(target)>0, 'empty download'
  print(json.dumps({'saved':os.path.relpath(target,'/workspace'),'bytes':os.path.getsize(target),'suggested_filename':download.suggested_filename}))
`, string(args))
	case "download":
		name, _ := input["path"].(string)
		destination, err := sandbox.WorkspacePath(name)
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		url, _ := input["url"].(string)
		selector, _ := input["selector"].(string)
		if url == "" && selector == "" {
			return &ToolResult{Error: "url or image selector is required"}, nil
		}
		args, _ := json.Marshal(map[string]string{"path": destination, "url": url, "selector": selector})
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
import json,base64,os
args=json.loads(%q)
target=os.path.realpath(args['path'])
assert target.startswith('/workspace/') and not any(p.startswith('.') for p in os.path.relpath(target,'/workspace').split('/')), 'invalid destination'
with sync_playwright() as p:
 browser=p.chromium.connect_over_cdp('http://127.0.0.1:9222',timeout=15000)
 page=browser.contexts[0].pages[0]
 url=args['url']
 if not url:
  el=page.locator(args['selector']).first
  url=el.evaluate('(el) => el.currentSrc || el.src || el.href')
 assert url, 'selected element has no downloadable URL'
 if url.startswith(('blob:','data:')):
  encoded=page.evaluate('''async ({url,format}) => {
   let response;
   try { response=await fetch(url); } catch (_) {
    const image=Array.from(document.images).find(i=>(i.currentSrc===url || i.src===url) && i.complete && i.naturalWidth);
    if(!image) throw new Error('Image is no longer available. Select a loaded image.');
    if(image.naturalWidth*image.naturalHeight>25000000) throw new Error('Image is too large');
    const canvas=document.createElement('canvas');canvas.width=image.naturalWidth;canvas.height=image.naturalHeight;
    canvas.getContext('2d').drawImage(image,0,0);return canvas.toDataURL(format,1).split(',')[1];
   }
   if(!response.ok) throw new Error('Download failed');
   const blob=await response.blob(); if(blob.size>50*1024*1024) throw new Error('File exceeds 50 MB');
   return await new Promise((resolve,reject)=>{const r=new FileReader();r.onload=()=>resolve(r.result.split(',')[1]);r.onerror=reject;r.readAsDataURL(blob);});
  }''',{'url':url,'format':'image/jpeg' if target.lower().endswith(('.jpg','.jpeg')) else 'image/png'})
  data=base64.b64decode(encoded)
 else:
  assert url.startswith(('http://','https://')), 'unsupported download URL'
  response=browser.contexts[0].request.get(url,timeout=30000)
  assert response.ok, 'download HTTP '+str(response.status)
  data=response.body()
 assert len(data)<=50*1024*1024, 'File exceeds 50 MB'
 os.makedirs(os.path.dirname(target),exist_ok=True)
 with open(target,'wb') as f: f.write(data)
 print(json.dumps({'saved':os.path.relpath(target,'/workspace'),'bytes':len(data),'message':'Available in chat files. Use this file instead of generating it again.'}))
`, string(args))
	case "navigate":
		url, _ := input["url"].(string)
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://127.0.0.1:9222")
    page = (browser.contexts[0].pages[0] if browser.contexts[0].pages else browser.contexts[0].new_page()) if browser.contexts else browser.new_context().new_page()
    page.goto(%q, wait_until="domcontentloaded", timeout=30000)
    print("Navigated to:", page.url)
    print("Title:", page.title())
`, url)

	case "evaluate":
		code, _ := input["script"].(string)
		if code == "" {
			return &ToolResult{Error: "script is required"}, nil
		}
		encoded, _ := json.Marshal(code)
		script = "from playwright.sync_api import sync_playwright\nimport json\nwith sync_playwright() as p:\n    browser = p.chromium.connect_over_cdp('http://127.0.0.1:9222')\n    page = browser.contexts[0].pages[0]\n    print(json.dumps(page.evaluate(json.loads(" + fmt.Sprintf("%q", string(encoded)) + ")), default=str))\n"
	case "get_text":
		selector, _ := input["selector"].(string)
		if selector == "" {
			selector = "body"
		}
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://127.0.0.1:9222")
    page = browser.contexts[0].pages[0]
    el = page.query_selector(%q)
    print(el.inner_text() if el else "Element not found")
`, selector)

	case "click":
		selector, _ := input["selector"].(string)
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://127.0.0.1:9222")
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
    browser = p.chromium.connect_over_cdp("http://127.0.0.1:9222")
    page = browser.contexts[0].pages[0]
    page.fill(%q, %q)
    print("Typed into:", %q)
`, selector, text, selector)

	case "screenshot":
		script = `
from playwright.sync_api import sync_playwright
import base64
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://127.0.0.1:9222")
    page = browser.contexts[0].pages[0]
    data = page.screenshot()
    print(base64.b64encode(data).decode())
`

	case "scroll":
		dir, _ := input["direction"].(string)
		delta := 500
		if dir == "up" {
			delta = -500
		}
		script = fmt.Sprintf(`
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp("http://127.0.0.1:9222")
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
	if res.ExitCode != 0 {
		return &ToolResult{Error: fmt.Sprintf("exit code %d: %s", res.ExitCode, res.Stderr)}, nil
	}
	return &ToolResult{Output: out}, nil
}
