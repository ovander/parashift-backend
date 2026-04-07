CREATE TABLE IF NOT EXISTS rules (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    type        TEXT        NOT NULL,
    configuration JSONB,
    severity    TEXT        NOT NULL DEFAULT 'WARNING',
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_rules_tenant_id ON rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rules_tenant_enabled ON rules(tenant_id, enabled) WHERE deleted_at IS NULL;
