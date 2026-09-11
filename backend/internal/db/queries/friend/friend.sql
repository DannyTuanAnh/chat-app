-- name: AddFriendById :one
select f.status::boolean, f.message::text from send_friend_request(sqlc.arg(sender_user_id), sqlc.arg(receiver_user_id)) as f;

-- name: GetPendingFriendRequests :many
select request_id, sender_id, send_at
from friend_requests
where receiver_id = sqlc.arg(current_user_id) and is_accepted = false and request_id > sqlc.arg(last_request_id)
order by send_at desc
limit 10;


-- name: GetSentFriendRequests :many
select request_id, receiver_id, send_at
from friend_requests
where sender_id = sqlc.arg(current_user_id) and is_accepted = false and request_id > sqlc.arg(last_request_id)
order by send_at desc
limit 10;

-- -- name: GetFriendsList :many
-- with friend_ids as (
--     select f.user1_id as id from friendships f where f.user2_id = $1
--     union all
--     select f.user2_id as id from friendships f where f.user1_id = $1
-- )

-- select 
--     u.uuid, 
--     u.user_id,
--     coalesce(p.name, u.display_name) as name, 
--     p.avatar_url,
--     u.is_active
-- from users u
-- left join profiles p on u.user_id = p.user_id
-- join friend_ids f on u.user_id = f.id
-- order by coalesce(p.name, u.display_name);

-- -- name: SearchFriendByName :many
-- with friend_ids as (
--     select f.user1_id as id from friendships f where f.user2_id = $1
--     union all
--     select f.user2_id as id from friendships f where f.user1_id = $1
-- )

-- select 
--     u.uuid, 
--     u.user_id,
--     coalesce(p.name, u.display_name) as name, 
--     p.avatar_url,
--     u.is_active
-- from users u
-- left join profiles p on u.user_id = p.user_id
-- join friend_ids f on u.user_id = f.id
-- where coalesce(p.name, u.display_name) ilike '%' || $2 || '%' 
-- order by coalesce(p.name, u.display_name);

-- name: RejectFriendRequestById :execresult
delete from friend_requests
where request_id = $1 and receiver_id = $2 and is_accepted=false;

-- name: GetInfoRelationship :one
-- Get user info with friendship/friend request status
select     
    -- Friend request status (if exists)
    fr.sender_id,
    fr.is_accepted,
    
    -- Friendship status (if exists)
    (f.user1_id is not null)::boolean as is_friend

from (select 1) as dummy

-- Check if there's a pending/accepted friend request
left join friend_requests fr 
    on (fr.sender_id = sqlc.arg(current_user_id) and fr.receiver_id = sqlc.arg(target_user_id))
    or (fr.sender_id = sqlc.arg(target_user_id) and fr.receiver_id = sqlc.arg(current_user_id))

-- Check if already friends
left join friendships f
    on (f.user1_id = LEAST(sqlc.arg(current_user_id), sqlc.arg(target_user_id)) and f.user2_id = GREATEST(sqlc.arg(current_user_id), sqlc.arg(target_user_id)))

where sqlc.arg(target_user_id) <> sqlc.arg(current_user_id);  

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