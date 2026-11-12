-- +goose Up
ALTER TABLE tasks
ALTER COLUMN created_at TYPE TIMESTAMPTZ
USING created_at AT TIME ZONE 'Europe/Vilnius';

ALTER TABLE tasks
ALTER COLUMN updated_at TYPE TIMESTAMPTZ
USING updated_at AT TIME ZONE 'Europe/Vilnius';

-- +goose Down
ALTER TABLE tasks
ALTER COLUMN created_at TYPE TIMESTAMP
USING created_at AT TIME ZONE 'Europe/Vilnius';

ALTER TABLE tasks
ALTER COLUMN updated_at TYPE TIMESTAMP
USING updated_at AT TIME ZONE 'Europe/Vilnius';