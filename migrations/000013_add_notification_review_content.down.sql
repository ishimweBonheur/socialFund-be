ALTER TABLE notifications
    DROP COLUMN IF EXISTS reject_url,
    DROP COLUMN IF EXISTS approve_url,
    DROP COLUMN IF EXISTS proof_url,
    DROP COLUMN IF EXISTS attachment_key;
