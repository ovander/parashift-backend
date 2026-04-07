ALTER TABLE employees DROP COLUMN IF EXISTS claim_token;
DROP INDEX IF EXISTS idx_employees_claim_token;
DROP INDEX IF EXISTS idx_employees_auth_id;
CREATE UNIQUE INDEX idx_employees_auth_id ON employees(auth_id);
ALTER TABLE employees ALTER COLUMN auth_id SET NOT NULL;
ALTER TABLE employees ADD CONSTRAINT uq_employee_auth_id UNIQUE (auth_id);
