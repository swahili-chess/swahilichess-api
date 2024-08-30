-- name: CreateUser :one
INSERT INTO users 
    (
     username, 
     full_name,
     lichess_username,
     chesscom_username,
     phone_number,
     photo,
     passcode,
     password_hash, 
     activated,
     enabled
    )
VALUES ($1, $2, $3 ,$4, $5, $6, $7, $8,$9, $10) RETURNING id, phone_number;


-- name: GetUserByPasscode :one
SELECT id, username, full_name, lichess_username, chesscom_username,
phone_number, photo, passcode, password_hash, enabled, activated, created_at
FROM users
WHERE passcode = $1;

-- name: GetUserById :one
SELECT id, username, full_name, lichess_username, chesscom_username,
phone_number, photo, passcode, password_hash, enabled, activated, created_at
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, full_name, lichess_username, chesscom_username,
phone_number, photo, passcode, password_hash, enabled, activated, created_at
FROM users
WHERE username = $1;

-- name: GetUserByToken :one
SELECT users.id, users.username, users.full_name, users.lichess_username, 
users.chesscom_username, users.phone_number,users.photo, users.passcode, users.password_hash, users.activated,users.enabled, users.created_at
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
    photo = $6,
    passcode = $7,
    password_hash = $8, 
    activated = $9,
    enabled = $10
WHERE 
    id = $11;

-- name: GetUserByUsernameOrPhone :one
SELECT * FROM users 
WHERE 
    (phone_number = $1 OR $1 = '' ) 
    AND 
    (username = $2 OR $2 = '');
    

-- name: DeleteUserById :exec
DELETE FROM users WHERE id = $1;



