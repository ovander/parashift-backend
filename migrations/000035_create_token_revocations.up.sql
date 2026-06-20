-- Token revocation floor (SEC: instant session revocation, backendkit v1.8.0).
-- One row per Socrate subject (sub). Any access token whose "iat" (issued-at) is
-- at or before revoked_after is rejected by the auth middleware, so logout /
-- password-change / admin-revoke take effect before the token's own expiry.
-- Re-authenticating mints a token with a later iat, which passes again.
CREATE TABLE IF NOT EXISTS token_revocations (
    sub           TEXT        PRIMARY KEY,
    revoked_after TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
