CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    contribution_id UUID REFERENCES contributions(id) ON DELETE RESTRICT,
    assistance_request_id UUID REFERENCES assistance_requests(id) ON DELETE RESTRICT,
    type VARCHAR(40) NOT NULL CHECK (type IN ('ACCOUNT_CREATED','CONTRIBUTION_DUE','CONTRIBUTION_OVERDUE','PROOF_SUBMITTED','CONTRIBUTION_APPROVED','CONTRIBUTION_REJECTED','ASSISTANCE_REQUESTED','ASSISTANCE_APPROVED','ASSISTANCE_REJECTED','ASSISTANCE_PAID')),
    channel VARCHAR(20) NOT NULL CHECK (channel IN ('EMAIL')), recipient VARCHAR(320) NOT NULL,
    subject TEXT, message TEXT, status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PROCESSING','SENT','FAILED')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0), last_error TEXT, next_retry_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (contribution_id IS NULL OR assistance_request_id IS NULL)
);
CREATE INDEX notifications_user_id_idx ON notifications (user_id);
CREATE INDEX notifications_contribution_id_idx ON notifications (contribution_id) WHERE contribution_id IS NOT NULL;
CREATE INDEX notifications_assistance_request_id_idx ON notifications (assistance_request_id) WHERE assistance_request_id IS NOT NULL;
CREATE INDEX notifications_pending_created_idx ON notifications (created_at) WHERE status = 'PENDING';
CREATE INDEX notifications_failed_retry_idx ON notifications (next_retry_at) WHERE status = 'FAILED';
CREATE INDEX notifications_reminder_dedupe_idx ON notifications (contribution_id, type, created_at DESC) WHERE type IN ('CONTRIBUTION_DUE','CONTRIBUTION_OVERDUE');
