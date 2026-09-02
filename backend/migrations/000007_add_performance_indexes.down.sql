-- Revert 000007: restore the original index set.

DROP INDEX IF EXISTS idx_notes_user_created;
DROP INDEX IF EXISTS idx_notes_user_visible;
CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes (user_id);

DROP INDEX IF EXISTS idx_task_types_user_created;
CREATE INDEX IF NOT EXISTS idx_task_types_user_id ON task_types (user_id);

DROP INDEX IF EXISTS idx_tasks_attachments_gin;

DROP INDEX IF EXISTS idx_tasks_reminder_pending;
CREATE INDEX idx_tasks_reminder_pending
    ON tasks (reminder_time)
    WHERE reminder_sent = false AND reminder_time IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_date ON tasks (date);
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks (user_id);
