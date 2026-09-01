UPDATE contributions
SET status = COALESCE(frozen_status, 'DUE'), frozen_status = NULL
WHERE status = 'FROZEN';

ALTER TABLE contributions DROP COLUMN frozen_status;
ALTER TABLE contributions DROP CONSTRAINT contributions_status_check;
ALTER TABLE contributions
    ADD CONSTRAINT contributions_status_check
    CHECK (status IN ('UPCOMING','DUE','OVERDUE','PENDING','APPROVED','REJECTED'));
