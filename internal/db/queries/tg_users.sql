-- name: InsertTgBotUsers :exec
INSERT INTO tgbot_users (id, isactive) VALUES ($1, $2);

-- name: UpdateTgBotUsers :exec
UPDATE tgbot_users SET isactive = $1 WHERE id = $2;

-- name: GetActiveTgBotUsers :many
SELECT id from tgbot_users WHERE isactive = true;