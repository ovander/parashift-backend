-- User interface locale preference stored on the employee record.
-- Valid values: 'fr' (default) | 'en'. Enforced as a check constraint.
ALTER TABLE employees ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT 'fr'
  CONSTRAINT employees_locale_check CHECK (locale IN ('fr', 'en'));
