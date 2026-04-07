CREATE TABLE availabilities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    time_ranges JSONB,
    note        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT uq_availability_employee_date UNIQUE (employee_id, date, deleted_at)
);
CREATE INDEX idx_availabilities_tenant_id ON availabilities(tenant_id);
CREATE INDEX idx_availabilities_employee_id ON availabilities(employee_id);
CREATE INDEX idx_availabilities_date ON availabilities(date);
