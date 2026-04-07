CREATE TABLE shift_instances (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    date                   DATE NOT NULL,
    start_time             TEXT NOT NULL,
    end_time               TEXT NOT NULL,
    role                   TEXT,
    required_qualification TEXT,
    source                 TEXT NOT NULL DEFAULT 'MANUAL' CHECK (source IN ('TEMPLATE', 'OVERRIDE', 'MANUAL')),
    source_template_id     UUID,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at             TIMESTAMPTZ
);
CREATE INDEX idx_shift_instances_tenant_id ON shift_instances(tenant_id);
CREATE INDEX idx_shift_instances_date ON shift_instances(date);
CREATE INDEX idx_shift_instances_tenant_date ON shift_instances(tenant_id, date);
