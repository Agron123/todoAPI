-- name: GetTaskByID :one
SELECT * from tasks
WHERE id = $1;