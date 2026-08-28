INSERT INTO users (id, full_name, email, phone, role, status)
VALUES (
    '8f332250-fec0-4bf8-9d73-320738b65d2e',
    'Ishim Webonheur',
    'ishimwebonheur078@gmail.com',
    'test-admin-078',
    'ADMIN',
    'INACTIVE'
)
ON CONFLICT (lower(email)) DO UPDATE
SET role = 'ADMIN',
    status = CASE
        WHEN users.google_id IS NULL THEN 'INACTIVE'
        ELSE 'ACTIVE'
    END,
    updated_at = NOW();
