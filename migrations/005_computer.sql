ALTER TABLE users ADD COLUMN IF NOT EXISTS computer_last_active TIMESTAMPTZ;
DELETE FROM global_settings WHERE key='omniroute_url';
