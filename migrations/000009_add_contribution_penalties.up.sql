ALTER TABLE contribution_plans
    ADD COLUMN late_fee_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN late_fee_percentage NUMERIC(5,2),
    ADD COLUMN grace_period_days INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT contribution_plans_late_fee_percentage_check CHECK (late_fee_percentage IS NULL OR late_fee_percentage BETWEEN 0 AND 100),
    ADD CONSTRAINT contribution_plans_grace_period_days_check CHECK (grace_period_days >= 0),
    ADD CONSTRAINT contribution_plans_late_fee_configuration_check CHECK (NOT late_fee_enabled OR late_fee_percentage IS NOT NULL);

ALTER TABLE contributions
    ADD COLUMN late_fee_percentage NUMERIC(5,2),
    ADD COLUMN late_fee_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN overdue_at TIMESTAMPTZ,
    ADD COLUMN approval_token_action VARCHAR(10),
    ADD CONSTRAINT contributions_late_fee_percentage_check CHECK (late_fee_percentage IS NULL OR late_fee_percentage BETWEEN 0 AND 100),
    ADD CONSTRAINT contributions_late_fee_amount_check CHECK (late_fee_amount >= 0),
    ADD CONSTRAINT contributions_approval_token_action_check CHECK (approval_token_action IS NULL OR approval_token_action IN ('APPROVE','REJECT'));

CREATE INDEX contributions_reminder_candidates_idx ON contributions (status, due_date) WHERE status IN ('OVERDUE', 'REJECTED');
