-- Phase 7 additions

-- Skills (saved successful workflows)
CREATE TABLE IF NOT EXISTS skills (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    content     TEXT NOT NULL,  -- markdown workflow
    tags        TEXT[],
    use_count   INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- MCP servers configured per user
CREATE TABLE IF NOT EXISTS mcp_servers (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    transport   TEXT NOT NULL DEFAULT 'http',  -- http | stdio
    url         TEXT,
    command     TEXT,
    args        TEXT[],
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, name)
);

-- Usage tracking (token/task stats)
CREATE TABLE IF NOT EXISTS usage_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    agent_id        UUID REFERENCES agents(id) ON DELETE SET NULL,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    event_type      TEXT NOT NULL,  -- llm_call | tool_call | task_complete
    model           TEXT,
    prompt_tokens   INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    tool_name       TEXT,
    duration_ms     INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS usage_events_user_idx ON usage_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS usage_events_agent_idx ON usage_events(agent_id, created_at DESC);
