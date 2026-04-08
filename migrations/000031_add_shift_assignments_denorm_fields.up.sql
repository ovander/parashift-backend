-- Denormalized shift date/time fields on shift_assignments — used to avoid
-- joining shift_instances on every assignment query.
ALTER TABLE shift_assignments ADD COLUMN IF NOT EXISTS shift_date       DATE NOT NULL DEFAULT '0001-01-01';
ALTER TABLE shift_assignments ADD COLUMN IF NOT EXISTS shift_start_time TEXT NOT NULL DEFAULT '';
ALTER TABLE shift_assignments ADD COLUMN IF NOT EXISTS shift_end_time   TEXT NOT NULL DEFAULT '';
