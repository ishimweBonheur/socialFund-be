DROP INDEX IF EXISTS contributions_reminder_candidates_idx;
ALTER TABLE contributions
    DROP CONSTRAINT IF EXISTS contributions_approval_token_action_check,
    DROP CONSTRAINT IF EXISTS contributions_late_fee_amount_check,
    DROP CONSTRAINT IF EXISTS contributions_late_fee_percentage_check,
    DROP COLUMN IF EXISTS approval_token_action,
    DROP COLUMN IF EXISTS overdue_at,
    DROP COLUMN IF EXISTS late_fee_amount,
    DROP COLUMN IF EXISTS late_fee_percentage;
ALTER TABLE contribution_plans
    DROP CONSTRAINT IF EXISTS contribution_plans_late_fee_configuration_check,
    DROP CONSTRAINT IF EXISTS contribution_plans_grace_period_days_check,
    DROP CONSTRAINT IF EXISTS contribution_plans_late_fee_percentage_check,
    DROP COLUMN IF EXISTS grace_period_days,
    DROP COLUMN IF EXISTS late_fee_percentage,
    DROP COLUMN IF EXISTS late_fee_enabled;
