-- name: GetAllTasksByStatus :many
SELECT * FROM tasks
WHERE status = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;