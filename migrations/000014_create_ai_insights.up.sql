CREATE TABLE IF NOT EXISTS ai_insights (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    type             TEXT        NOT NULL,
    message          TEXT        NOT NULL,
    recommendation   TEXT        NOT NULL DEFAULT '',
    confidence       FLOAT       NOT NULL DEFAULT 0,
    related_shift_id UUID        REFERENCES shift_instances(id) ON DELETE SET NULL,
    related_emp_id   UUID        REFERENCES employees(id) ON DELETE SET NULL,
    dismissed        BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_insights_tenant_id ON ai_insights(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ai_insights_tenant_dismissed ON ai_insights(tenant_id, dismissed) WHERE deleted_at IS NULL;
