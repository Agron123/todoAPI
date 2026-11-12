-- name: GetAllTasksByStatus :many
SELECT * FROM tasks
WHERE status = $1;