CREATE TABLE IF NOT EXISTS qualifications (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    name             TEXT        NOT NULL,
    issuing_body     TEXT        NOT NULL DEFAULT '',
    required_for_role TEXT       NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_qualifications_tenant_id ON qualifications(tenant_id);

CREATE TABLE IF NOT EXISTS employee_qualifications (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    employee_id      UUID        NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    qualification_id UUID        NOT NULL REFERENCES qualifications(id) ON DELETE CASCADE,
    issue_date       DATE,
    expiry_date      DATE,
    verified         BOOLEAN     NOT NULL DEFAULT FALSE,
    document_url     TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_employee_qualifications_tenant_id       ON employee_qualifications(tenant_id);
CREATE INDEX IF NOT EXISTS idx_employee_qualifications_employee_id     ON employee_qualifications(employee_id);
CREATE INDEX IF NOT EXISTS idx_employee_qualifications_qualification_id ON employee_qualifications(qualification_id);
