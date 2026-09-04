CREATE TABLE
    fund_transactions (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
        type VARCHAR(20) NOT NULL CHECK (type IN ('CONTRIBUTION', 'ADJUSTMENT', 'REFUND')),
        direction VARCHAR(3) NOT NULL CHECK (direction IN ('IN', 'OUT')),
        amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
        contribution_id UUID REFERENCES contributions (id) ON DELETE RESTRICT,
        reference VARCHAR(255),
        description TEXT,
        recorded_by UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        CHECK (
            (
                type = 'CONTRIBUTION'
                AND direction = 'IN'
                AND contribution_id IS NOT NULL
            )
            OR type IN ('ADJUSTMENT', 'REFUND')
        )
    );

CREATE UNIQUE INDEX fund_transactions_one_contribution_idx ON fund_transactions (contribution_id)
WHERE
    type = 'CONTRIBUTION';

CREATE UNIQUE INDEX fund_transactions_reference_unique ON fund_transactions (reference)
WHERE
    reference IS NOT NULL;

CREATE INDEX fund_transactions_user_id_idx ON fund_transactions (user_id);

CREATE INDEX fund_transactions_type_idx ON fund_transactions (type);

CREATE INDEX fund_transactions_direction_created_idx ON fund_transactions (direction, created_at);

CREATE INDEX fund_transactions_created_at_idx ON fund_transactions (created_at);

CREATE INDEX fund_transactions_contribution_id_idx ON fund_transactions (contribution_id)
WHERE
    contribution_id IS NOT NULL;

CREATE TABLE payment_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    account_name VARCHAR(120) NOT NULL,
    payment_type VARCHAR(20) NOT NULL CHECK (payment_type IN ('PHONE','MERCHANT')),
    phone_number VARCHAR(30), merchant_code VARCHAR(60),
    ussd_template VARCHAR(200) NOT NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((payment_type='PHONE' AND phone_number IS NOT NULL) OR (payment_type='MERCHANT' AND merchant_code IS NOT NULL))
);
INSERT INTO payment_settings(id,account_name,payment_type,phone_number,ussd_template)
VALUES(1,'Social Fund','PHONE','0784963589','*182*1*1*{phone_number}#');
