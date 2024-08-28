-- name: CreateUser :exec
INSERT INTO users 
    (
     username, 
     full_name,
     lichess_username,
     chesscom_username,
     phone_number,
     password_hash, 
     activated
    )
VALUES ($1, $2, $3 ,$4, $5, $6, $7);


-- name: GetUserByPhone :one
SELECT id, username, full_name, lichess_username, chesscom_username,
phone_number, password_hash, activated, created_at
FROM users
WHERE phone_number = $1;


-- name: GetUserByUsername :one
SELECT id, username, full_name, lichess_username, chesscom_username,
phone_number, password_hash, activated, created_at
FROM users
WHERE username = $1;


-- name: GetUserByToken :one
SELECT users.id, users.username, users.full_name, users.lichess_username, 
users.chesscom_username, users.phone_number, users.password_hash, users.activated, users.created_at
FROM users
INNER JOIN token
ON users.id = token.user_id
WHERE token.hash = $1
AND token.scope = $2
AND token.expiry > $3;


-- name: UpdateUserById :exec
UPDATE users
SET 
    username = $1, 
    full_name = $2, 
    lichess_username = $3, 
    chesscom_username = $4, 
    phone_number = $5, 
    password_hash = $6, 
    activated = $7
WHERE 
    id = $8;


-- name: DeleteUserById :exec
DELETE FROM users WHERE id = $1;



