-- SEC-6: track when each invite (claim) token was issued so the claim flow can
-- enforce a TTL. Nullable; null falls back to created_at for legacy rows.
ALTER TABLE employees ADD COLUMN IF NOT EXISTS claim_token_issued_at TIMESTAMPTZ;
