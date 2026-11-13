-- name: UpdateTaskByID :one
UPDATE tasks
SET title = $1, status = $2, description = $3, updated_at = NOW()
WHERE id = $4
RETURNING *;