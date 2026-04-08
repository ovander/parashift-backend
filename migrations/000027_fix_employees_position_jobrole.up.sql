-- Rename the original 'role' column to 'position' to match the Go model field.
-- 'position' controls RBAC access: 'manager' | 'employee'.
ALTER TABLE employees RENAME COLUMN role TO position;

-- Add 'job_role' which controls shift eligibility (pharmacist, animator, etc.).
-- Separate concern from 'position' — an employee can be a manager with job_role 'pharmacist'.
ALTER TABLE employees ADD COLUMN IF NOT EXISTS job_role TEXT NOT NULL DEFAULT '';
