ALTER TABLE employees ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_employees_email ON employees(email) WHERE email <> '';
