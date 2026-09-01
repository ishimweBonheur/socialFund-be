ALTER TABLE contributions DROP CONSTRAINT contributions_status_check;
ALTER TABLE contributions
    ADD CONSTRAINT contributions_status_check
    CHECK (status IN ('UPCOMING','DUE','OVERDUE','PENDING','APPROVED','REJECTED','FROZEN'));

ALTER TABLE contributions
    ADD COLUMN frozen_status VARCHAR(20)
    CHECK (frozen_status IN ('UPCOMING','DUE','OVERDUE','PENDING','REJECTED'));

UPDATE contributions c
SET frozen_status = c.status,
    status = 'FROZEN',
    updated_at = NOW()
FROM users u
WHERE u.id = c.user_id
  AND u.status = 'SUSPENDED'
  AND c.status IN ('UPCOMING','DUE','OVERDUE','PENDING','REJECTED');
