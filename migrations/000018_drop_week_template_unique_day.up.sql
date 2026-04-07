-- Allow multiple shifts per day per employee (split shifts, e.g. 9-12 and 13-19).
-- The old unique index prevented saving more than one WeekTemplate row for the
-- same (employee_id, week_type, day_of_week) triple. Drop it and replace with a
-- plain (non-unique) index so the delete-then-insert upsert pattern still has
-- efficient lookups while allowing any number of rows per day.

DROP INDEX IF EXISTS idx_week_template_emp_type_day;

CREATE INDEX IF NOT EXISTS idx_week_template_emp_type_day
    ON week_templates (employee_id, week_type, day_of_week);
