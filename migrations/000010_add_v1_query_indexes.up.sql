CREATE INDEX users_status_role_created_idx ON users (status, role, created_at DESC);
CREATE INDEX notifications_status_type_created_idx ON notifications (status, type, created_at DESC);
CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);
