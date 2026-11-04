-- +goose Up
CREATE TABLE tasks(
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT DEFAULT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT progress_status CHECK (status IN ('todo', 'in_progress', 'done'))
);

-- +goose Down
DROP TABLE tasks;