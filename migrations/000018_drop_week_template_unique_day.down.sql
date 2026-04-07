-- Restore the unique index (reverting split-shift support).
-- WARNING: this will fail if any employee already has multiple shifts on the same day.

DROP INDEX IF EXISTS idx_week_template_emp_type_day;

CREATE UNIQUE INDEX idx_week_template_emp_type_day
    ON week_templates (employee_id, week_type, day_of_week);
