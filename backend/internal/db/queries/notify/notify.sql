-- name: SaveUserStatus :execresult
insert into user_status (
    user_id,
    status,
    updated_at
)
values ($1, $2, $3)
on conflict (user_id)
do update set
    status = excluded.status,
    updated_at = excluded.updated_at
where user_status.updated_at < excluded.updated_at;
    
-- name: DeleteUserStatus :exec
delete from user_status where user_id = $1;

-- name: GetDisabledUser :one
select user_id, status, updated_at from user_status where user_id = $1;

-- name: CreateFCMNotification :execresult
insert into user_info(
    user_id,
    push_token,
    device_uuid
)
values ($1, $2, $3)
on conflict (user_id, device_uuid)
do update set
    push_token = excluded.push_token,
    updated_at = now();

-- name: GetUserInfo :one
select user_id, device_uuid, push_token, created_at, updated_at from user_info where user_id = $1 and device_uuid = $2;

-- name: GetUserInfoByUserID :many
select push_token, device_uuid from user_info where user_id = $1;