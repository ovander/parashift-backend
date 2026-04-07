-- Revert: drop status column and restore original source check constraint.
ALTER TABLE shift_instances DROP COLUMN IF EXISTS status;

ALTER TABLE shift_instances
    DROP CONSTRAINT IF EXISTS shift_instances_source_check;

ALTER TABLE shift_instances
    ADD CONSTRAINT shift_instances_source_check
    CHECK (source IN ('TEMPLATE', 'OVERRIDE', 'MANUAL'));
