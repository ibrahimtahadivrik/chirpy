-- name: CreateRefreshToken :one
insert into refresh_tokens(token, created_at, updated_at, user_id, expires_at, revoked_at)
values(
    $1,
    now(),
    now(),
    $2,
    $3,
    null
)
returning *;

-- name: GetUserFromRefreshToken :one
select * from refresh_tokens
where token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(),
    updated_at = NOW()
WHERE token = $1;