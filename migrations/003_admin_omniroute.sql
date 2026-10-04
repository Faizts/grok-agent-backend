-- Migration 003: Admin Panel, Budgets, Per-User Models & OmniRoute

ALTER TABLE users
ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user',
ADD COLUMN IF NOT EXISTS monthly_budget_usd NUMERIC(10,2) DEFAULT 10.00,
ADD COLUMN IF NOT EXISTS spent_this_month_usd NUMERIC(10,4) DEFAULT 0.0000,
ADD COLUMN IF NOT EXISTS allowed_models TEXT[] DEFAULT '{"gpt-4o", "gemini-2.0-flash", "claude-3.5-sonnet", "antigravity", "codex"}',
ADD COLUMN IF NOT EXISTS assigned_model TEXT DEFAULT 'gpt-4o',
ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'active';

-- Model rates for automatic cost calculation per 1K tokens
CREATE TABLE IF NOT EXISTS model_rates (
    model TEXT PRIMARY KEY,
    input_cost_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0.001500,
    output_cost_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0.006000,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Default seed rates
INSERT INTO model_rates (model, input_cost_per_1k, output_cost_per_1k) VALUES
    ('gpt-4o', 0.002500, 0.010000),
    ('gpt-4o-mini', 0.000150, 0.000600),
    ('gemini-2.0-flash', 0.000100, 0.000400),
    ('claude-3.5-sonnet', 0.003000, 0.015000),
    ('antigravity', 0.001500, 0.006000),
    ('codex', 0.002000, 0.008000),
    ('openrouter/auto', 0.001000, 0.004000)
ON CONFLICT (model) DO NOTHING;

-- Global settings (OmniRoute endpoint, default budget, etc.)
CREATE TABLE IF NOT EXISTS global_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO global_settings (key, value) VALUES
    ('omniroute_url', 'http://omniroute:8000/v1'),
    ('default_user_budget', '10.00'),
    ('default_user_model', 'gpt-4o')
ON CONFLICT (key) DO NOTHING;
