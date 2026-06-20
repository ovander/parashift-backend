-- SEC-4: remediate invite-token / auth-subject leakage (issue #4).
--
-- Companion to SEC-2 (which stops future leakage). This migration cleans up
-- data that may already have leaked and invalidates any token that could have
-- been harvested.

-- 1) Scrub secrets that were serialized into audit_logs before SEC-2.
--    Pre-fix, Employee was marshaled with Go field names, so the JSON keys are
--    "AuthID" and "ClaimToken". Strip both (plus their lowercase json-tag
--    variants, for safety) from every audit row's before/after payload.
--    Idempotent: re-running is a no-op once the keys are gone.
UPDATE audit_logs
SET after = (after - 'ClaimToken' - 'AuthID' - 'claim_token' - 'auth_id')
WHERE after IS NOT NULL
  AND (after ? 'ClaimToken' OR after ? 'AuthID'
       OR after ? 'claim_token' OR after ? 'auth_id');

UPDATE audit_logs
SET before = (before - 'ClaimToken' - 'AuthID' - 'claim_token' - 'auth_id')
WHERE before IS NOT NULL
  AND (before ? 'ClaimToken' OR before ? 'AuthID'
       OR before ? 'claim_token' OR before ? 'auth_id');

-- 2) Invalidate every outstanding (unclaimed) invite token by rotating it to a
--    fresh random value, so any previously-leaked token can no longer be
--    claimed. Affected invitees must be re-sent an invite via the admin
--    "resend invite" flow (which rotates the token and emails a new link).
UPDATE employees
SET claim_token = gen_random_uuid()::text
WHERE claim_token IS NOT NULL
  AND deleted_at IS NULL;
