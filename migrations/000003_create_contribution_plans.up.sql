CREATE TABLE
    contribution_plans (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
        amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
        frequency VARCHAR(20) NOT NULL CHECK (
            frequency IN ('DAILY', 'WEEKLY', 'MONTHLY', 'CUSTOM')
        ),
        interval_value INTEGER CHECK (interval_value > 0),
        due_day INTEGER CHECK (due_day BETWEEN 1 AND 31),
        start_date DATE NOT NULL,
        end_date DATE,
        reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
        reminder_frequency VARCHAR(20) CHECK (
            reminder_frequency IN ('DAILY', 'WEEKLY', 'MONTHLY', 'CUSTOM')
        ),
        reminder_interval INTEGER CHECK (reminder_interval > 0),
        pre_due_reminder_enabled BOOLEAN NOT NULL DEFAULT FALSE,
        pre_due_reminder_frequency VARCHAR(20) CHECK (
            pre_due_reminder_frequency IN ('DAILY', 'WEEKLY', 'MONTHLY', 'CUSTOM')
        ),
        pre_due_reminder_interval INTEGER CHECK (pre_due_reminder_interval > 0),
        pre_due_reminder_days_before_due INTEGER NOT NULL DEFAULT 3 CHECK (pre_due_reminder_days_before_due >= 0),
        is_active BOOLEAN NOT NULL DEFAULT TRUE,
        late_fee_enabled BOOLEAN NOT NULL DEFAULT FALSE,
        late_fee_percentage NUMERIC(5, 2) CHECK (
            late_fee_percentage IS NULL
            OR late_fee_percentage BETWEEN 0 AND 100
        ),
        grace_period_days INTEGER NOT NULL DEFAULT 0 CHECK (grace_period_days >= 0),
        created_by UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        CHECK (
            end_date IS NULL
            OR end_date >= start_date
        ),
        CHECK (
            (
                frequency = 'MONTHLY'
                AND due_day IS NOT NULL
            )
            OR (frequency <> 'MONTHLY')
        ),
        CHECK (
            (
                frequency = 'CUSTOM'
                AND interval_value IS NOT NULL
            )
            OR frequency <> 'CUSTOM'
        ),
        CHECK (
            (NOT reminder_enabled)
            OR reminder_frequency IS NOT NULL
        ),
        CHECK (
            (
                reminder_frequency = 'CUSTOM'
                AND reminder_interval IS NOT NULL
            )
            OR reminder_frequency IS DISTINCT
            FROM
                'CUSTOM'
        ),
        CHECK (
            (NOT pre_due_reminder_enabled)
            OR pre_due_reminder_frequency IS NOT NULL
        ),
        CHECK (
            (
                pre_due_reminder_frequency = 'CUSTOM'
                AND pre_due_reminder_interval IS NOT NULL
            )
            OR pre_due_reminder_frequency IS DISTINCT
            FROM
                'CUSTOM'
        ),
        CHECK (
            NOT late_fee_enabled
            OR late_fee_percentage IS NOT NULL
        )
    );

CREATE INDEX contribution_plans_user_id_idx ON contribution_plans (user_id);

CREATE UNIQUE INDEX contribution_plans_one_active_per_user_idx ON contribution_plans (user_id)
WHERE
    is_active;