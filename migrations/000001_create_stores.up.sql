CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE stores (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    opening_hours JSONB,
    timezone   TEXT NOT NULL DEFAULT 'Europe/Brussels',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
