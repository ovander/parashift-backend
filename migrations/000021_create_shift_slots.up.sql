CREATE TABLE IF NOT EXISTS shift_slots (
    id                        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                 UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    scheme                    TEXT        NOT NULL,          -- 'A' | 'B'
    day_of_week               INTEGER     NOT NULL,          -- 1=Monday … 7=Sunday
    start_time                TEXT        NOT NULL,          -- HH:MM
    end_time                  TEXT        NOT NULL,          -- HH:MM
    required_role             TEXT        NOT NULL,
    required_qualification_id UUID        REFERENCES qualifications(id) ON DELETE SET NULL,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_shift_slots_tenant_id     ON shift_slots(tenant_id);
CREATE INDEX IF NOT EXISTS idx_shift_slots_tenant_scheme ON shift_slots(tenant_id, scheme) WHERE deleted_at IS NULL;
