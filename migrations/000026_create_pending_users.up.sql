-- pending_users holds auth_id of users who logged in but haven't been provisioned
-- with an Employee record yet. Upserted on each /me call, deleted on activation.
CREATE TABLE IF NOT EXISTS pending_users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_id    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_pending_users_auth_id UNIQUE (auth_id)
);
