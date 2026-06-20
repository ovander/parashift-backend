-- Optimistic concurrency control (ARC-3): a monotonically increasing version per
-- row, guarded on update so a stale writer is rejected instead of clobbering.
-- Applied to the high-concurrency tables migrated in this change.
ALTER TABLE employees         ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE shift_instances   ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE shift_assignments ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE leave_requests    ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE swap_requests     ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 0;
