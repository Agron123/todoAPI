-- name: CreateTask :one
INSERT INTO tasks (id, title, description, status, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    sqlc.narg('description'),
    $2,
    NOW(),
    NOW()
)
RETURNING *;