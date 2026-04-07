CREATE TABLE coverage_requirements (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    day_of_week    INT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    start_time     TEXT NOT NULL,
    end_time       TEXT NOT NULL,
    min_staff      INT NOT NULL DEFAULT 1 CHECK (min_staff >= 1),
    required_role  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);
CREATE INDEX idx_coverage_requirements_tenant_id ON coverage_requirements(tenant_id);
