-- Migration: add 'SLOT' to the shift_instances.source check constraint,
-- and add the status column if not already present.
--
-- The original source constraint was created inline in migration 000006 without
-- an explicit name, so PostgreSQL assigned one automatically. We find it via
-- pg_constraint and drop it before adding the updated version.

DO $$
DECLARE
    v_constraint TEXT;
BEGIN
    -- Find the check constraint on shift_instances.source (unnamed, auto-generated name).
    SELECT conname
      INTO v_constraint
      FROM pg_constraint
      JOIN pg_class ON pg_class.oid = pg_constraint.conrelid
     WHERE pg_class.relname = 'shift_instances'
       AND pg_constraint.contype = 'c'
       AND pg_get_constraintdef(pg_constraint.oid) LIKE '%source%'
     LIMIT 1;

    IF v_constraint IS NOT NULL THEN
        EXECUTE format('ALTER TABLE shift_instances DROP CONSTRAINT %I', v_constraint);
    END IF;
END $$;

-- Add the updated constraint that includes 'SLOT'.
ALTER TABLE shift_instances
    ADD CONSTRAINT shift_instances_source_check
    CHECK (source IN ('TEMPLATE', 'OVERRIDE', 'MANUAL', 'SLOT'));

-- Add status column if it doesn't already exist (AutoMigrate may have added it in dev).
ALTER TABLE shift_instances
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'DRAFT';

-- Add / replace status check constraint the same way.
DO $$
DECLARE
    v_constraint TEXT;
BEGIN
    SELECT conname
      INTO v_constraint
      FROM pg_constraint
      JOIN pg_class ON pg_class.oid = pg_constraint.conrelid
     WHERE pg_class.relname = 'shift_instances'
       AND pg_constraint.contype = 'c'
       AND pg_get_constraintdef(pg_constraint.oid) LIKE '%status%'
     LIMIT 1;

    IF v_constraint IS NOT NULL THEN
        EXECUTE format('ALTER TABLE shift_instances DROP CONSTRAINT %I', v_constraint);
    END IF;
END $$;

ALTER TABLE shift_instances
    ADD CONSTRAINT shift_instances_status_check
    CHECK (status IN ('DRAFT', 'PUBLISHED'));
