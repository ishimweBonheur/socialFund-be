CREATE TABLE fund_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    type VARCHAR(20) NOT NULL CHECK (type IN ('CONTRIBUTION','ADJUSTMENT','REFUND')),
    direction VARCHAR(3) NOT NULL CHECK (direction IN ('IN','OUT')),
    amount NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    contribution_id UUID REFERENCES contributions(id) ON DELETE RESTRICT,
    reference VARCHAR(255), description TEXT, recorded_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((type = 'CONTRIBUTION' AND direction = 'IN' AND contribution_id IS NOT NULL) OR type IN ('ADJUSTMENT','REFUND'))
);
CREATE UNIQUE INDEX fund_transactions_one_contribution_idx ON fund_transactions (contribution_id) WHERE type = 'CONTRIBUTION';
CREATE UNIQUE INDEX fund_transactions_reference_unique ON fund_transactions (reference) WHERE reference IS NOT NULL;
CREATE INDEX fund_transactions_user_id_idx ON fund_transactions (user_id);
CREATE INDEX fund_transactions_type_idx ON fund_transactions (type);
CREATE INDEX fund_transactions_direction_created_idx ON fund_transactions (direction, created_at);
CREATE INDEX fund_transactions_created_at_idx ON fund_transactions (created_at);
CREATE INDEX fund_transactions_contribution_id_idx ON fund_transactions (contribution_id) WHERE contribution_id IS NOT NULL;
