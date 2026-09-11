-- name: GetProfile :one
SELECT p.*, u.uuid
FROM profiles p
join users u on p.user_id = u.user_id
WHERE p.user_id = $1 AND u.is_active = true;

-- name: GetProfileByUserId :one
SELECT 
    u.user_id,
    p.name,
    p.birthday,
    p.avatar_url,
    p.avatar_version
FROM profiles p
join users u on p.user_id = u.user_id
WHERE p.user_id = sqlc.arg(target_user_id) AND u.is_active = true AND p.user_id <> sqlc.arg(current_user_id);

-- name: GetProfileByUserIDs :many
SELECT 
    u.user_id,
    p.name,
    p.avatar_url,
    p.avatar_version
FROM profiles p
join users u on p.user_id = u.user_id
WHERE p.user_id = ANY(sqlc.arg(user_ids)::int4[]) AND u.is_active = true;

-- name: CreateProfile :one
INSERT INTO profiles (user_id, name, email, birthday, avatar_url) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: UpdateProfileByUserId :one
UPDATE profiles p
SET 
    name = COALESCE(sqlc.narg('name'), p.name),
    birthday = COALESCE(sqlc.narg('birthday'), p.birthday),
    phone = COALESCE(sqlc.narg('phone'), p.phone),
    updated_at = now()
FROM users u 
WHERE p.user_id = u.user_id AND p.user_id = $1 AND u.is_active = true
RETURNING p.user_id, u.uuid, p.name, p.email, p.phone, p.birthday, p.avatar_url, p.avatar_version, p.updated_at;

-- name: UpdateProfileAvatarByUserId :one
WITH old AS (
    SELECT avatar_url FROM profiles WHERE user_id = $1
)

UPDATE profiles p
SET
    avatar_url = $2,
    avatar_version = p.avatar_version + CASE
        WHEN old.avatar_url IS DISTINCT FROM $2 THEN 1
        ELSE 0
    END,
    updated_at = now()

FROM old
JOIN users u ON p.user_id = u.user_id AND u.is_active = true
WHERE p.user_id = $1
RETURNING p.user_id, u.uuid, p.name, p.email, p.phone, p.birthday, p.avatar_url, p.avatar_version, p.updated_at;

