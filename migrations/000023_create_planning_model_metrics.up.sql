CREATE TABLE IF NOT EXISTS planning_model_metrics (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    store_id         UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    model_scheme     TEXT        NOT NULL,          -- 'A', 'B', or custom name
    week_start       DATE        NOT NULL,
    coverage_rate    FLOAT       NOT NULL DEFAULT 0,
    overtime_hours   FLOAT       NOT NULL DEFAULT 0,
    adjustment_count INTEGER     NOT NULL DEFAULT 0,
    violation_count  INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_planning_model_metrics_tenant_id    ON planning_model_metrics(tenant_id);
CREATE INDEX IF NOT EXISTS idx_planning_model_metrics_store_id     ON planning_model_metrics(store_id);
CREATE INDEX IF NOT EXISTS idx_planning_model_metrics_model_scheme ON planning_model_metrics(model_scheme);
