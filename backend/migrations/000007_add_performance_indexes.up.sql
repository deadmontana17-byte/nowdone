-- Performance indexes for the hot read paths (task list, note list, task-type
-- list) and the attachment-cleanup containment check.
--
-- Plain CREATE INDEX (not CONCURRENTLY) so the file runs inside golang-migrate's
-- transaction; the tables are small enough that the brief write lock is fine.

-- notes: ListByUser always filters user_id and returns rows ORDER BY created_at
-- DESC. A composite index serves both the filter and the ordering (no sort
-- node). Supersedes the plain idx_notes_user_id.
CREATE INDEX IF NOT EXISTS idx_notes_user_created ON notes (user_id, created_at DESC);
DROP INDEX IF EXISTS idx_notes_user_id;

-- notes: the default list (PIN not entered) also filters is_hidden = false.
-- A partial index keeps it small and covers the common case exactly.
CREATE INDEX IF NOT EXISTS idx_notes_user_visible
    ON notes (user_id, created_at DESC)
    WHERE is_hidden = false;

-- task_types: ListByUser filters user_id and returns ORDER BY created_at.
-- Supersedes the plain idx_task_types_user_id.
CREATE INDEX IF NOT EXISTS idx_task_types_user_created ON task_types (user_id, created_at);
DROP INDEX IF EXISTS idx_task_types_user_id;

-- tasks: TaskRepository.OtherTaskReferencesURL runs
--   attachments @> '[{"url": "..."}]'::jsonb
-- to check whether another task still points at a file before deleting it from
-- S3. Without an index this is a per-user sequential scan. jsonb_path_ops is the
-- smallest GIN opclass and supports the @> operator we use.
CREATE INDEX IF NOT EXISTS idx_tasks_attachments_gin
    ON tasks USING gin (attachments jsonb_path_ops);

-- tasks: make the reminder-dispatch partial index match the worker query
-- exactly. DuePendingReminders also filters is_done = false, so folding it into
-- the predicate shrinks the index and lets it satisfy the query on its own.
DROP INDEX IF EXISTS idx_tasks_reminder_pending;
CREATE INDEX idx_tasks_reminder_pending
    ON tasks (reminder_time)
    WHERE reminder_sent = false AND reminder_time IS NOT NULL AND is_done = false;

-- tasks: idx_tasks_date (date alone) and idx_tasks_user_id are both redundant.
-- Every task query filters user_id first (and usually a date range too), so the
-- composite idx_tasks_user_date serves them by leftmost prefix; the reminder
-- scan uses idx_tasks_reminder_pending; single-row ops use the primary key.
-- Dropping the two removes write amplification on the busiest table.
DROP INDEX IF EXISTS idx_tasks_date;
DROP INDEX IF EXISTS idx_tasks_user_id;
