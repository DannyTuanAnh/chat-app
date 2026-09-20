-- name: FindExistingIdentity :one
select user_id, status, revoked_at from auth_identities 
where provider = $1 and provider_user_id = $2;

-- name: CreateIdentity :exec
insert into auth_identities (user_id, provider, provider_user_id, email)
values ($1, $2, $3, $4);

-- name: ActiveIdentity :execresult
update auth_identities 
set status = 'active', revoked_at = null 
where provider = $1 
and provider_user_id = $2
and revoked_at > now() - interval '30 days';

-- name: DisableIdentity :execresult
update auth_identities set status = 'revoked', revoked_at = now() where provider = $1 and user_id = $2;

-- name: CreateSession :one
insert into sessions (user_id, device_id) values ($1,sqlc.arg(device_id)::uuid) returning session_id;

-- name: CreateDevice :execresult
insert into devices (device_id, user_id) 
select sqlc.arg(device_id)::uuid, $1
where (
    select count(*)
    from devices 
    where user_id = $1 and revoked_at is null
) < 3;

-- name: CheckSession :one
select 
    s.session_id,
    s.user_id,
    s.device_id,
    s.revoked
from sessions s
join devices d on s.device_id = d.device_id and d.user_id = s.user_id
where s.session_id = sqlc.arg(session_id)::uuid
    and s.device_id = sqlc.arg(device_id)::uuid;

-- name: RevokeSessionAndDevice :exec
with revoked_device as (
    update devices d
    set d.revoked_at = now(),
        d.last_seen_at = now()
    where d.device_id = $2 
)
update sessions s
set s.revoked = true, s.revoke_at = now() 
where s.session_id = $1;

-- name: RevokeAllSessions :exec
with revoked_devices as (
    update devices d
    set d.revoked_at = now(),
        d.last_seen_at = now()
    where d.user_id = $1
)
update sessions s
set s.revoked = true, s.revoke_at = now()
where s.user_id = $1;

-- name: CleanupSessionTable :exec
delete from sessions where revoked = true and revoke_at < now() - interval '1 days';

-- manage apikeys
-- name: CreateAPIKey :exec
insert into api_keys (key_hash) values ($1);

-- name: RevokeAPIKeyByKey :exec
update api_keys set is_active = false, revoked_at = now() where key_hash = $1;

-- name: RevokeAllAPIKeys :exec
update api_keys set is_active = false, revoked_at = now() where is_active = true;

-- name: ValidateAPIKey :one
select is_active from api_keys where key_hash = $1;