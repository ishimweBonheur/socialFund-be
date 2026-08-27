CREATE TABLE contributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    contribution_plan_id UUID NOT NULL REFERENCES contribution_plans(id) ON DELETE RESTRICT,
    expected_amount NUMERIC(15,2) NOT NULL CHECK (expected_amount > 0), due_date DATE NOT NULL,
    paid_amount NUMERIC(15,2) CHECK (paid_amount >= 0), payment_date TIMESTAMPTZ,
    payment_method VARCHAR(30) CHECK (payment_method IN ('MOBILE_MONEY','BANK_TRANSFER','CASH','OTHER')),
    transaction_reference VARCHAR(255), proof_url TEXT, proof_uploaded_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'UPCOMING' CHECK (status IN ('UPCOMING','DUE','OVERDUE','PENDING','APPROVED','REJECTED')),
    rejection_reason TEXT, approved_by UUID REFERENCES users(id) ON DELETE RESTRICT, approved_at TIMESTAMPTZ,
    approval_token_hash TEXT, approval_token_expires_at TIMESTAMPTZ, approval_token_used_at TIMESTAMPTZ,
    notes TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (contribution_plan_id, due_date),
    CHECK (status <> 'APPROVED' OR (approved_by IS NOT NULL AND approved_at IS NOT NULL)),
    CHECK (status <> 'REJECTED' OR (rejection_reason IS NOT NULL AND length(btrim(rejection_reason)) > 0))
);
CREATE UNIQUE INDEX contributions_transaction_reference_unique ON contributions (transaction_reference) WHERE transaction_reference IS NOT NULL;
CREATE INDEX contributions_user_due_date_idx ON contributions (user_id, due_date DESC);
CREATE INDEX contributions_plan_id_idx ON contributions (contribution_plan_id);
CREATE INDEX contributions_status_due_date_idx ON contributions (status, due_date);
CREATE INDEX contributions_pending_created_idx ON contributions (created_at) WHERE status = 'PENDING';
