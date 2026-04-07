DROP INDEX IF EXISTS idx_employees_email;
ALTER TABLE employees DROP COLUMN IF EXISTS email;
