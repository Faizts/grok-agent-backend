CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

-- Users
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email       TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Agents (Bots)
CREATE TABLE IF NOT EXISTS agents (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    system_prompt   TEXT,
    status          TEXT DEFAULT 'idle',
    sandbox_id      TEXT,
    novnc_port      TEXT,
    vnc_port        TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Conversations
CREATE TABLE IF NOT EXISTS conversations (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT DEFAULT 'New conversation',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    content         TEXT,
    tool_name       TEXT,
    tool_input      JSONB,
    tool_result     JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Memory (vector store)
CREATE TABLE IF NOT EXISTS memories (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    store       TEXT NOT NULL,
    content     TEXT NOT NULL,
    embedding   vector(1536),
    metadata    JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS memories_embedding_idx
    ON memories USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Tool approvals
CREATE TABLE IF NOT EXISTS approvals (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    tool_name   TEXT NOT NULL,
    action      TEXT NOT NULL,
    status      TEXT DEFAULT 'pending',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Always-allow tool rules per agent
CREATE TABLE IF NOT EXISTS tool_rules (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    tool_name   TEXT NOT NULL,
    rule        TEXT NOT NULL DEFAULT 'always_allow',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(agent_id, tool_name)
);
