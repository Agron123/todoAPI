-- name: GetAllTasks :many
SELECT * FROM tasks
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;