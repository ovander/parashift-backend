ALTER TABLE audit_logs
  ADD COLUMN IF NOT EXISTS resource_type TEXT    NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS resource_id   TEXT    NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action        ON audit_logs(action);
