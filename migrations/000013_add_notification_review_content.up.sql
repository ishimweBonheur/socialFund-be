ALTER TABLE notifications
    ADD COLUMN attachment_key TEXT,
    ADD COLUMN proof_url TEXT,
    ADD COLUMN approve_url TEXT,
    ADD COLUMN reject_url TEXT;
