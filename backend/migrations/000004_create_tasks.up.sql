CREATE TABLE tasks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type_id          UUID REFERENCES task_types(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    description      JSONB NOT NULL DEFAULT '{}'::jsonb,
    attachments      JSONB NOT NULL DEFAULT '[]'::jsonb,
    date             DATE NOT NULL,
    is_done          BOOLEAN NOT NULL DEFAULT false,
    reminder_time    TIMESTAMPTZ,
    reminder_sent    BOOLEAN NOT NULL DEFAULT false,
    is_recurring     BOOLEAN NOT NULL DEFAULT false,
    recurrence_rule  JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_user_id ON tasks (user_id);
CREATE INDEX idx_tasks_date ON tasks (date);
CREATE INDEX idx_tasks_user_date ON tasks (user_id, date);
CREATE INDEX idx_tasks_reminder_pending ON tasks (reminder_time) WHERE reminder_sent = false AND reminder_time IS NOT NULL;
