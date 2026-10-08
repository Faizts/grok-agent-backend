# GrokAgent

Named AI agents with persistent memory, tools, and a shared computer per user. The user UI and admin UI are separate Next.js 16.3.8 / React 19 apps; the backend is Go 1.25 / Gin.

## Start

```sh
cp .env.example .env
# Set POSTGRES_PASSWORD, JWT_SECRET, LLM_API_KEY and your provider/model.
docker compose up --build
```

User app: http://localhost:3000. Admin app: http://localhost:3001. API: http://localhost:8080/api/v1. The first registered user is administrator. Both apps use the same account system.

Compose builds the sandbox and runs PostgreSQL 16 with pgvector, SearXNG, the Go backend, and both web apps. There is no bundled LLM gateway or Redis service. Set LLM_BASE_URL to your OpenAI-compatible provider. Every assigned model needs a model_rates entry and streaming token usage support.

## Shared computer and resource use

New agents share one Docker computer per user. Conversations, prompts, memories and approvals remain per agent; files and browser sign-ins are shared across that user's agents. Users have separate containers/volumes. A PostgreSQL advisory lock allows one computer turn per user across conversations and backend replicas; competing sends get a retryable busy error.

The sandbox uses Debian bookworm-slim, Openbox, Xvfb, Chromium/Playwright, xterm and noVNC. It includes Python, Requests, BeautifulSoup and Pillow. NumPy/Pandas/Matplotlib are optional: set SANDBOX_DATA_PACKAGES=true when building, or install them on demand. Only visible Chromium is downloaded; the unused headless-shell binary is omitted. It avoids a full desktop environment. New computers default to a 1 CPU / 1024 MB limit and 256 MB shared memory. Set SANDBOX_CPUS and SANDBOX_MEMORY_MB before provisioning to change these limits. Browser-heavy tasks may need more RAM; these are configured caps, not measured usage.

Computers are created on first message and stop after SANDBOX_IDLE_MINUTES (default 10). Set 0 to disable idle stopping. The next message restarts the computer. Workspace files and Chromium's profile live in /workspace on a persistent Docker volume, surviving stops and container replacement. The browser is visible through Show Desktop for sign-in and manual interaction. Idle shutdown also ends an unattended desktop session; send a message to wake it.

Existing per-agent containers continue to be used to preserve old workspaces. New agents use the shared computer. Upgrading the image does not replace existing containers automatically. Back up old workspace volumes before manually migrating/recreating them. Deleting a legacy agent removes its computer and workspace; deleting the last agent removes the user's shared computer and workspace. Other agent deletions keep the shared computer intact.

Desktop ports bind to host loopback. Remote deployment needs an authenticated TLS desktop proxy or tunnel; do not expose unauthenticated VNC publicly. Browser API/WebSocket/desktop addresses are supplied using NEXT_PUBLIC_* build arguments in Compose. The admin maps host 3001 to container 3000. Rebuild web apps when public URLs change. WebSocket origins must use the same hostname as the backend (different ports are allowed).

## Runtime

REST manages agents, conversations, memory, skills, approvals, MCP servers, usage and administration. WebSocket /api/v1/ws/:conversation_id streams text, tool calls/results, approvals, done and errors. User messages are saved before execution; tool calls and results retain matching IDs. Disconnect cancels the active turn; reconnect reloads saved history without resending messages.

The loop retrieves memories and matching skills, discovers Streamable HTTP MCP tools, streams an OpenAI-compatible model, checks budgets, and executes up to 100 iterations per message. Shell, Python, browser, file writes/deletes and MCP tools require approval. File read/list and web search are automatic. Always allow is persistent per agent and tool. Successful tool workflows are captured as skills and solution memories.

Browser supports navigate, click, type, get_text, screenshot, scroll and evaluate. File actions are confined to /workspace. MCP stdio is not supported. Model prices are admin configured per 1000 tokens. Monthly budgets reset in UTC; the latest call can exceed the remaining budget because cost is known after completion. Embedding requests are not included in chat metering.

## Code map

- cmd/server and internal/api: startup, routes and handlers.
- internal/agent: loop, events, approval enforcement and history.
- internal/sandbox and sandbox/: Docker computers and Linux image.
- internal/tools, llm and mcp: built-in tools and provider adapters.
- internal/memory, skills and usage: recall, workflows and billing.
- migrations/: idempotent schema changes run by Compose before backend startup.
- ../grok-agent-frontend: user interface.
- ../grok-agent-admin: administration interface.

## Checks

```sh
go test ./...
# In each web app:
npm run test
npm run typecheck
npm run lint
npm run build
```

Unit tests use temporary local HTTP servers. Live approval/database, browser, noVNC and Docker provisioning need infrastructure verification.

## Grok Bot reference and remaining scope

The shared computer model follows https://docs.x.ai/grok-bot/overview and https://prod.cursor.com/docs/grok-bot/work (reviewed 2026-10-06). This implementation keeps persistent bot memory, skills, approvals and a visible shared browser, with serialized computer work for lower resource use. Separate parallel bot screens, background turns after disconnect, scheduled routines, bot-to-bot handoffs, file attachments and demonstration learning are not implemented yet.

## Docker build hygiene

All four Dockerfiles are used: API, user UI, admin UI, and sandbox. Their .dockerignore files allow only required source/configuration/assets. Local executables, Git metadata, environment files, dependencies, tests, logs and documentation stay out of build contexts. The web runtime images contain standalone Next.js output and public assets; the API runtime contains its compiled server. Keep the PostgreSQL data volume and agent workspace volumes when removing old containers.

Persistent Compose service logs are capped at three 10 MB files. The migration and sandbox-image helper services use temporary mounts instead of leaving anonymous data volumes after each run. Images built from these Dockerfiles carry io.grokagent.project=grokagent labels for identification. Shared Docker build caches are not deleted automatically because they can be used by other projects.

### Chat files and browser downloads

Each reply shows image previews and download buttons for files created or changed in that turn. Snapshots are saved under `/workspace/agents/<agent-id>/responses/<turn-id>/`, and the agent’s Files panel lists its folder (up to 50 MB per file, 300 listed files). Bots share the computer but have separate output folders and file views. Authenticated `GET /api/v1/agents/:id/files` lists files; `GET /api/v1/agents/:id/files/download?path=generated/image.png` streams a file after checking agent ownership and workspace confinement. Hidden directories, browser profiles and symlink escapes are excluded. Opening Files or downloading a file wakes a sleeping computer on demand.

The browser `download` action accepts `path` and either `selector` (an image or link) or `url`. Blob images are saved directly; a loaded image can be exported through a canvas if its blob cannot be fetched. This avoids sending binary data through model history or running a temporary receiver server. The chat keeps tool activity collapsed and offers Continue after reaching the 100-round limit. The agent receives a reminder during its final five rounds to save completed work and summarize what remains.

Attachment metadata is persisted on the user turn, so files stay with the corresponding reply after refresh and subsequent messages. Unchanged working files are not repeated in a new response. Previous response snapshots preserve their contents after working files are overwritten.

## Media workflows

All agents can read six embedded Markdown skills from `internal/skills/bundles/`: video-production, browser-media-generation, ffmpeg-video-editing, transcription-subtitles, capcut-browser-editing and social-publishing. The read-only `skill` tool loads complete workflows/resources on demand, avoiding partial 3,000-character instructions. New computers include FFmpeg/ffprobe; existing computers need these installed separately if absent. The renderer uses Python standard library and bounded sequential FFmpeg jobs. Browser `upload` selects a workspace file on an observed file input; `download_click` captures native exports. Verify site upload completion separately.

The Skills page lists built-ins and supports importing Markdown up to 64 KB through New Skill. Imports remain editable before saving and are available to the user's agents. Saved conversation workflows are reference data, never authorization to act. Media service availability depends on the user's signed-in account. Workflows ask for missing provider/editor preferences, checkpoint assets, and prepare reviewed uploads to YouTube, Facebook Pages and Instagram. Public/scheduled publishing requires explicit approval of the concrete asset and destination. No posting or service credentials are bundled.

SearXNG's `SEARXNG_SECRET` is optional. If unset or empty, its startup wrapper creates a random 32-byte secret in the dedicated `searxng_secret` volume and reuses it across restarts/recreation. An explicitly configured value takes precedence. Keep this volume alongside other deployment data; removing it rotates the generated key. No secret is committed to the source checkout or reused from JWT/database credentials.
