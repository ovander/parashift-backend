ALTER TABLE employees         DROP COLUMN IF EXISTS version;
ALTER TABLE shift_instances   DROP COLUMN IF EXISTS version;
ALTER TABLE shift_assignments DROP COLUMN IF EXISTS version;
ALTER TABLE leave_requests    DROP COLUMN IF EXISTS version;
ALTER TABLE swap_requests     DROP COLUMN IF EXISTS version;
