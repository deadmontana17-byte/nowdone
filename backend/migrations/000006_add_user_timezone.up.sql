-- Per-user IANA timezone (e.g. "Europe/Moscow"). Reminder times entered without
-- an offset are interpreted in this zone; existing rows default to UTC, which
-- matches the previous behaviour.
ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';
