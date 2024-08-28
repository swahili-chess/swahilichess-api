-- name: CreateToken :exec
INSERT INTO token (hash, user_id, expiry, scope)
VALUES ($1, $2, $3, $4);

-- name: DeleteToken :exec
DELETE FROM token WHERE token.hash = $1 and user_id = $2;
