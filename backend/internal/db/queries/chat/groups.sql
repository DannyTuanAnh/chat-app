-- name: AddGroupMembers :one
insert into conversation_members (conversation_id, user_id, role)
select $1, $2, 'member'
where exists (
    select 1 from conversation_members cm
    where cm.conversation_id = $1 and cm.user_id = $3 and cm.role = 'admin'
)
returning conversation_id, user_id, role;

-- name: RemoveGroupMembers :exec
delete from conversation_members cm
where cm.conversation_id = $1 and cm.user_id = $2
and exists (
    select 1 from conversation_members cm
    where cm.conversation_id = $1 and cm.user_id = $3 and cm.role = 'admin'
);

-- name: LeaveConversation :one
delete from conversation_members
where conversation_id = $1 and user_id = $2
returning conversation_id, user_id;

-- -- name: GetGroupMembers :many
-- select 
--     u.uuid, 
--     coalesce(p.name, u.display_name) as name, 
--     p.avatar_url,
--     cm.role
-- from conversation_members cm
-- join users u on cm.user_id = u.user_id
-- left join profiles p on u.user_id = p.user_id
-- where cm.conversation_id = $1
-- order by coalesce(p.name, u.display_name);

-- name: UpdateGroupInfo :one
update groups set 
    name = coalesce(sqlc.narg('name'), name),
    avatar_url = coalesce(sqlc.narg('avatar_url'), avatar_url)
where conversation_id = $1
returning conversation_id, name, avatar_url, created_at;

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