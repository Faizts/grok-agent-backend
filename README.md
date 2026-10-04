# GrokAgent

A full-stack AI agent platform inspired by Grok Bot and Agent Zero.  
Each agent gets its own persistent Linux computer (Docker sandbox) with a real browser, shell, Python, and memory.

## Stack

| Layer | Tech |
|---|---|
| Backend | Go 1.25 (Gin, Gorilla WebSocket, Docker SDK) |
| Frontend | Next.js 15 (App Router, Tailwind CSS, Zustand) |
| Sandbox | Ubuntu 22.04 + XFCE4 + Playwright + noVNC |
| Database | PostgreSQL 16 + pgvector |
| Cache | Redis 7 |
| Search | SearXNG (private, self-hosted) |
| LLM | Any OpenAI-compatible API (OpenAI, Grok, Ollama) |

## Quick Start

### 1. Clone and configure

```bash
git clone <your-repo>
cd grok-agent
cp .env.example .env
# Edit .env — set LLM_API_KEY, LLM_MODEL, JWT_SECRET
```

### 2. Build the sandbox image (one time)

```bash
cd sandbox
docker build -t grok-agent-sandbox:latest .
cd ..
```

### 3. Start everything

```bash
docker compose up --build
```

- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- API health: http://localhost:8080/health

### 4. Register and chat

1. Open http://localhost:3000
2. Create an account
3. Click **New Agent** → name it → **Create & Chat**
4. Start chatting — the agent can run shell commands, Python, browse the web, and manage files

---

## Architecture

```
Browser (Next.js :3000)
    │
    ├── /chat          → Agent list, create agent
    ├── /chat/[id]     → Chat + Live Desktop split view
    │
    └── WebSocket ──── Go Backend (:8080)
                            │
                    ┌───────┴───────┐
                    │               │
              Agent Loop       REST API
              (Go goroutine)   /api/v1/...
                    │
          ┌─────────┼─────────┐
          │         │         │
       Docker    pgvector    LLM API
       Sandbox   Memory    (OpenAI compat)
       (Ubuntu)
       ├── bash shell
       ├── python3
       ├── playwright browser
       ├── XFCE4 desktop (noVNC)
       └── /workspace (persistent volume)
```

## Agent Tools

| Tool | What it does |
|---|---|
| `shell` | Execute bash commands in the sandbox |
| `python` | Run Python 3 code (NumPy, Pandas, Matplotlib pre-installed) |
| `browser` | Control a real Chromium browser (navigate, click, type, screenshot) |
| `file` | Read, write, list, delete files in `/workspace` |
| `web_search` | Search the web via local SearXNG instance |

## API Endpoints

### Auth
- `POST /api/v1/auth/register` — `{ email, password }` → `{ token, user_id }`
- `POST /api/v1/auth/login` — `{ email, password }` → `{ token, user_id }`

### Agents
- `GET /api/v1/agents` — list agents
- `POST /api/v1/agents` — `{ name, system_prompt? }` → create agent
- `GET /api/v1/agents/:id` — get agent
- `DELETE /api/v1/agents/:id` — delete agent

### Conversations
- `GET /api/v1/conversations` — list conversations
- `POST /api/v1/conversations` — `{ agent_id, title? }` → create conversation
- `GET /api/v1/conversations/:id/messages` — get message history

### WebSocket
- `GET /api/v1/ws/:conversation_id` — streaming agent events

Event types:
```json
{ "type": "text",        "content": "..." }
{ "type": "tool_call",   "tool": "shell", "input": { "command": "ls" } }
{ "type": "tool_result", "tool": "shell", "output": "..." }
{ "type": "done" }
{ "type": "error",       "content": "..." }
```

### Memory
- `GET /api/v1/agents/:id/memory?store=solutions` — list memories
- `DELETE /api/v1/agents/:id/memory/:mem_id` — delete memory

### Approvals
- `GET /api/v1/approvals` — pending approvals
- `POST /api/v1/approvals/:id/respond` — `{ decision: "approved"|"denied"|"always" }`

## LLM Providers

Switch providers with env vars — no code changes needed:

```env
# OpenAI
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL=gpt-4o

# Grok (xAI)
LLM_BASE_URL=https://api.x.ai/v1
LLM_MODEL=grok-4

# Local Ollama
LLM_BASE_URL=http://host.docker.internal:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=llama3.1
```

## Project Structure

```
grok-agent/
├── backend/
│   ├── cmd/server/main.go          # Entrypoint
│   ├── internal/
│   │   ├── agent/                  # Agent loop + event streaming
│   │   ├── api/                    # HTTP server + router
│   │   │   └── handlers/           # Auth, agents, conversations, WS, approvals, memory
│   │   ├── auth/                   # JWT + bcrypt
│   │   ├── config/                 # Env config
│   │   ├── db/                     # Postgres + Redis clients
│   │   ├── llm/                    # OpenAI-compatible LLM client
│   │   ├── memory/                 # pgvector memory store + embedder
│   │   ├── sandbox/                # Docker container management
│   │   └── tools/                  # shell, python, browser, file, web_search
│   └── migrations/001_init.sql
├── frontend/
│   └── src/
│       ├── app/
│       │   ├── page.tsx            # Login/register
│       │   ├── chat/page.tsx       # Agent list
│       │   └── chat/[id]/page.tsx  # Chat + desktop canvas
│       ├── components/chat/        # ChatWindow, MessageBubble, ToolCallCard
│       ├── lib/                    # API client, utils
│       └── store/                  # Zustand: auth, chat
├── sandbox/
│   ├── Dockerfile                  # Ubuntu + XFCE + Playwright + noVNC
│   ├── supervisord.conf
│   └── start.sh
├── docker-compose.yml
└── .env.example
```

## Development (without Docker)

```bash
# Start Postgres + Redis
docker compose up postgres redis -d

# Run backend
cd backend
export DATABASE_URL=postgres://grokagent:secret@localhost:5432/grokagent
export REDIS_URL=redis://localhost:6379
export LLM_API_KEY=your-key
export JWT_SECRET=dev-secret
go run ./cmd/server

# Run frontend
cd frontend
npm run dev
```

## Roadmap

- [ ] Approval workflow UI (Allow Once / Always / Deny banners)
- [ ] Memory dashboard (search, edit, delete stored memories)
- [ ] Agent skill saving (auto-save successful workflows as reusable skills)
- [ ] MCP client (connect external Model Context Protocol servers)
- [ ] Multi-agent coordination (agents can spawn sub-agents)
- [ ] Scheduled tasks (run agent tasks on cron schedules)
- [ ] Usage dashboard (token usage, tasks run, memory stats)
