DROP INDEX IF EXISTS users_status_changed_at_idx;
ALTER TABLE users DROP COLUMN IF EXISTS status_changed_at;
