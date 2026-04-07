-- Allow auth_id to be empty (for employees pending invite claim).
ALTER TABLE employees ALTER COLUMN auth_id SET DEFAULT '';
ALTER TABLE employees ALTER COLUMN auth_id DROP NOT NULL;

-- Drop the plain unique constraint on auth_id — empty strings must be allowed for
-- multiple unclaimed employees. Keep the index for fast lookups by sub claim.
ALTER TABLE employees DROP CONSTRAINT IF EXISTS uq_employee_auth_id;
DROP INDEX IF EXISTS idx_employees_auth_id;
CREATE UNIQUE INDEX idx_employees_auth_id ON employees(auth_id) WHERE auth_id <> '';

-- One-time invite / claim token (UUID string); NULL once claimed.
ALTER TABLE employees ADD COLUMN claim_token TEXT;
CREATE UNIQUE INDEX idx_employees_claim_token ON employees(claim_token) WHERE claim_token IS NOT NULL;
