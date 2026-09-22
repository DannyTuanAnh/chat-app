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

-- name: AcceptFriendRequestById :one
update friend_requests
set is_accepted = true
where request_id = $1 and receiver_id = $2 and is_accepted = false
returning sender_id, receiver_id, is_accepted;

-- name: CreateFriendShip :execresult
insert into friendships (user1_id, user2_id, established_at)
values (least(sqlc.arg(sender_user_id)::int, sqlc.arg(receiver_user_id)::int), greatest(sqlc.arg(sender_user_id)::int, sqlc.arg(receiver_user_id)::int), sqlc.arg(established_at))
on conflict(user1_id, user2_id)
do update set 
    established_at = excluded.established_at
where friendships.established_at < excluded.established_at;

-- name: GetFriendList :many
select 
    (case 
        when user1_id = sqlc.arg(current_user_id) then user2_id 
        else user1_id
    end)::int as friend_ids
from friendships
where 
    (user1_id = sqlc.arg(current_user_id) and user2_id > sqlc.arg(last_friend_id)) 
    or
    (user2_id = sqlc.arg(current_user_id) and user1_id > sqlc.arg(last_friend_id))
order by friend_ids
limit 10;

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