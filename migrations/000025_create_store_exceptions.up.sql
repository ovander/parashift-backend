CREATE TABLE IF NOT EXISTS store_exceptions (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    date       DATE        NOT NULL,
    type       TEXT        NOT NULL CHECK (type IN ('EXTRA_OPEN', 'FORCED_CLOSED')),
    note       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_store_exceptions_tenant_id ON store_exceptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_store_exceptions_date      ON store_exceptions(date);
