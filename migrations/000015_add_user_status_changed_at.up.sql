ALTER TABLE users ADD COLUMN status_changed_at TIMESTAMPTZ;

UPDATE users
SET status_changed_at = CASE WHEN status = 'ACTIVE' THEN created_at ELSE updated_at END;

ALTER TABLE users
    ALTER COLUMN status_changed_at SET DEFAULT NOW(),
    ALTER COLUMN status_changed_at SET NOT NULL;

CREATE INDEX users_status_changed_at_idx ON users (status, status_changed_at);
