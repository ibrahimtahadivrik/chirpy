-- name: CreateUser :one
insert into users(id, created_at, updated_at, email, hashed_password)
values(
    gen_random_uuid(),
    now(),
    now(),
    $1,
    $2
)
returning *;

-- name: DeleteUsers :exec
delete from users;

-- name: ValidateUser :one
select * from users
where email = $1;

-- name: ChangeEmailAndPassword :one
update users
set
    email = $2,
    hashed_password = $3,
    updated_at = now()
where id = $1
returning *;

-- name: ChangeChirpyRed :exec
update users
set
    is_chirpy_red = $2
where id = $1;