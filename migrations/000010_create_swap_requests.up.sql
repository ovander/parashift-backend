CREATE TABLE swap_requests (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    requester_id       UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    target_employee_id UUID REFERENCES employees(id),
    shift_instance_id  UUID NOT NULL REFERENCES shift_instances(id) ON DELETE CASCADE,
    target_shift_id    UUID REFERENCES shift_instances(id),
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    note               TEXT,
    reviewed_by        UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at         TIMESTAMPTZ
);
CREATE INDEX idx_swap_requests_tenant_id ON swap_requests(tenant_id);
CREATE INDEX idx_swap_requests_requester_id ON swap_requests(requester_id);
