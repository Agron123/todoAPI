-- name: DeleteTaskByID :one
DELETE FROM tasks
WHERE id = $1
RETURNING *;