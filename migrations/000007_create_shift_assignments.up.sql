CREATE TABLE shift_assignments (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    shift_instance_id UUID NOT NULL REFERENCES shift_instances(id) ON DELETE CASCADE,
    employee_id      UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    status           TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed', 'cancelled', 'pending')),
    assigned_by      UUID NOT NULL,
    assigned_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT uq_shift_employee UNIQUE (shift_instance_id, employee_id, deleted_at)
);
CREATE INDEX idx_shift_assignments_tenant_id ON shift_assignments(tenant_id);
CREATE INDEX idx_shift_assignments_shift_instance_id ON shift_assignments(shift_instance_id);
CREATE INDEX idx_shift_assignments_employee_id ON shift_assignments(employee_id);
