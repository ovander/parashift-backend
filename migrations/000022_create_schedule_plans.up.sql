CREATE TABLE IF NOT EXISTS schedule_plans (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    store_id     UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    week_start   DATE        NOT NULL,                          -- always a Monday
    state        TEXT        NOT NULL DEFAULT 'DRAFT'
                             CHECK (state IN ('DRAFT', 'PUBLISHED', 'LIVE', 'ARCHIVED')),
    published_at TIMESTAMPTZ,
    published_by UUID,
    snapshots    TEXT        NOT NULL DEFAULT '[]',             -- JSON []PlanSnapshot
    override_log TEXT        NOT NULL DEFAULT '[]',            -- JSON []OverrideEntry
    model_scheme TEXT        NOT NULL DEFAULT '',              -- 'A', 'B', or custom
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    CONSTRAINT uq_schedule_plan_store_week UNIQUE (store_id, week_start)
);

CREATE INDEX IF NOT EXISTS idx_schedule_plans_tenant_id  ON schedule_plans(tenant_id);
CREATE INDEX IF NOT EXISTS idx_schedule_plans_store_id   ON schedule_plans(store_id);
CREATE INDEX IF NOT EXISTS idx_schedule_plans_week_start ON schedule_plans(week_start);
