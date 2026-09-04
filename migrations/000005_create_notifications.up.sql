CREATE TABLE
    notifications (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
        contribution_id UUID REFERENCES contributions (id) ON DELETE RESTRICT,
        type VARCHAR(40) NOT NULL CHECK (
            type IN (
                'ACCOUNT_CREATED',
                'CONTRIBUTION_DUE',
                'CONTRIBUTION_OVERDUE',
                'PROOF_SUBMITTED',
                'CONTRIBUTION_APPROVED',
                'CONTRIBUTION_REJECTED',
                'SUPPORT_REQUEST',
                'SUPPORT_REQUEST_RECEIVED',
                'CONTRIBUTION_REVIEW_REMINDER'
            )
        ),
        channel VARCHAR(20) NOT NULL CHECK (channel IN ('EMAIL', 'IN_APP')),
        recipient VARCHAR(320) NOT NULL,
        subject TEXT,
        message TEXT,
        status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (
            status IN ('PENDING', 'PROCESSING', 'SENT', 'FAILED')
        ),
        attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
        last_error TEXT,
        next_retry_at TIMESTAMPTZ,
        sent_at TIMESTAMPTZ,
        read_at TIMESTAMPTZ,
        attachment_key TEXT,
        proof_url TEXT,
        approve_url TEXT,
        reject_url TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
    );

CREATE INDEX notifications_user_id_idx ON notifications (user_id);

CREATE INDEX notifications_contribution_id_idx ON notifications (contribution_id)
WHERE
    contribution_id IS NOT NULL;

CREATE INDEX notifications_pending_created_idx ON notifications (created_at)
WHERE
    status = 'PENDING';

CREATE INDEX notifications_failed_retry_idx ON notifications (next_retry_at)
WHERE
    status = 'FAILED';

CREATE INDEX notifications_reminder_dedupe_idx ON notifications (contribution_id, type, created_at DESC)
WHERE
    type IN ('CONTRIBUTION_DUE', 'CONTRIBUTION_OVERDUE');

CREATE INDEX notifications_user_unread_idx ON notifications (user_id, created_at DESC)
WHERE
    read_at IS NULL;

CREATE INDEX notifications_status_type_created_idx ON notifications (status, type, created_at DESC);

CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);
