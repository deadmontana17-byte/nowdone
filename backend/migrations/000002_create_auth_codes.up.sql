CREATE TABLE auth_codes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code             TEXT NOT NULL,
    purpose          TEXT NOT NULL CHECK (purpose IN ('auth', 'reset')),
    user_id          UUID REFERENCES users(id) ON DELETE CASCADE,
    telegram_chat_id BIGINT,
    is_used          BOOLEAN NOT NULL DEFAULT false,
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_codes_code ON auth_codes (code);
CREATE INDEX idx_auth_codes_expires_at ON auth_codes (expires_at);
