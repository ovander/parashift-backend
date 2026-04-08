-- public_holidays is global reference data (not tenant-scoped).
-- Populated from the French government API (calendrier.api.gouv.fr).
CREATE TABLE IF NOT EXISTS public_holidays (
    date DATE        NOT NULL,
    zone TEXT        NOT NULL DEFAULT 'metropole',
    name TEXT        NOT NULL,
    PRIMARY KEY (date, zone)
);

CREATE INDEX IF NOT EXISTS idx_public_holidays_zone ON public_holidays(zone);
