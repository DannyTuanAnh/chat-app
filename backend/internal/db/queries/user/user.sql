-- name: GetUserByUUID :one
SELECT 
    u.user_id,
    p.name,
    u.is_active,
    p.avatar_url,
    p.avatar_version
    
FROM users u

LEFT JOIN profiles p
    ON p.user_id = u.user_id

WHERE u.uuid = sqlc.arg(target_user_uuid); 

-- name: ActiveUser :execresult
UPDATE users SET is_active = true, disable_at = null 
WHERE user_id = $1
AND is_active = false
AND disable_at > now() - interval '30 days';

-- name: IsUserActive :one
SELECT is_active FROM users WHERE user_id = $1;

-- name: IsExistProfile :one
SELECT EXISTS (SELECT 1 FROM profiles WHERE user_id = $1);

-- name: GetUUIDByUserId :one
SELECT uuid FROM users WHERE user_id = $1;

-- name: CreateUser :one
INSERT INTO users (display_name) VALUES ($1) RETURNING user_id;

-- name: DeleteUser :execresult
DELETE FROM users WHERE user_id = $1;

-- name: DisableUser :exec
UPDATE users SET is_active = false, disable_at = now() WHERE user_id = $1;