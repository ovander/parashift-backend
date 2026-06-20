-- Irreversible by design (SEC-4): scrubbed audit secrets cannot be restored,
-- and rotated invite tokens must not be reverted to their leaked values.
-- This down migration is intentionally a no-op.
SELECT 1;
