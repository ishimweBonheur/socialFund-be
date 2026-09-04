DROP TRIGGER IF EXISTS audit_logs_immutable ON audit_logs;

DROP FUNCTION IF EXISTS prevent_audit_log_mutation ();

DROP TABLE IF EXISTS audit_logs;