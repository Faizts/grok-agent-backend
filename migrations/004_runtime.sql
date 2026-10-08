ALTER TABLE users ADD COLUMN IF NOT EXISTS budget_period DATE NOT NULL DEFAULT date_trunc('month', CURRENT_DATE)::date;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_call_id TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_calls JSONB;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS sequence BIGSERIAL;
CREATE INDEX IF NOT EXISTS messages_conversation_sequence_idx ON messages(conversation_id, sequence);
