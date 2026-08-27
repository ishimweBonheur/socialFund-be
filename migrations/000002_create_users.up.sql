CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), full_name VARCHAR(150) NOT NULL,
    email VARCHAR(255) NOT NULL, phone VARCHAR(30) NOT NULL, google_id VARCHAR(255),
    role VARCHAR(20) NOT NULL CHECK (role IN ('ADMIN','MEMBER')),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE','SUSPENDED')),
    last_login_at TIMESTAMPTZ, created_by UUID REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_unique UNIQUE (email), CONSTRAINT users_phone_unique UNIQUE (phone)
);
CREATE UNIQUE INDEX users_google_id_unique ON users (google_id) WHERE google_id IS NOT NULL;
CREATE UNIQUE INDEX users_email_case_insensitive_unique ON users (lower(email));
CREATE INDEX users_role_status_idx ON users (role, status);
