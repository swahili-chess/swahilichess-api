-- name: GetLichessTeamMembers :many
SELECT lichess_id from lichess;

-- name: InsertLichessTeamMember :exec
INSERT INTO lichess(lichess_id, username) VALUES ($1, $2);