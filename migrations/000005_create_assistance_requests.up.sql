CREATE TABLE assistance_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount_requested NUMERIC(15,2) NOT NULL CHECK (amount_requested > 0), reason VARCHAR(255) NOT NULL,
    description TEXT, attachment_url TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','REJECTED','PAID','CANCELLED')),
    amount_approved NUMERIC(15,2) CHECK (amount_approved >= 0 AND amount_approved <= amount_requested),
    reviewed_by UUID REFERENCES users(id) ON DELETE RESTRICT, reviewed_at TIMESTAMPTZ, rejection_reason TEXT,
    amount_disbursed NUMERIC(15,2) CHECK (amount_disbursed >= 0 AND amount_disbursed <= amount_approved),
    disbursement_method VARCHAR(30), disbursement_reference VARCHAR(255),
    disbursed_by UUID REFERENCES users(id) ON DELETE RESTRICT, disbursed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status NOT IN ('APPROVED','PAID') OR (amount_approved IS NOT NULL AND amount_approved > 0 AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)),
    CHECK (status <> 'REJECTED' OR (rejection_reason IS NOT NULL AND length(btrim(rejection_reason)) > 0 AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)),
    CHECK (status <> 'PAID' OR (amount_disbursed IS NOT NULL AND amount_disbursed > 0 AND disbursement_method IS NOT NULL AND disbursement_reference IS NOT NULL AND disbursed_by IS NOT NULL AND disbursed_at IS NOT NULL))
);
CREATE UNIQUE INDEX assistance_disbursement_reference_unique ON assistance_requests (disbursement_reference) WHERE disbursement_reference IS NOT NULL;
CREATE INDEX assistance_requests_user_id_idx ON assistance_requests (user_id);
CREATE INDEX assistance_requests_status_created_idx ON assistance_requests (status, created_at);
CREATE INDEX assistance_requests_created_at_idx ON assistance_requests (created_at);
