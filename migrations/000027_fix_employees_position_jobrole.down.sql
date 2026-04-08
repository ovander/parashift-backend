ALTER TABLE employees DROP COLUMN IF EXISTS job_role;
ALTER TABLE employees RENAME COLUMN position TO role;
