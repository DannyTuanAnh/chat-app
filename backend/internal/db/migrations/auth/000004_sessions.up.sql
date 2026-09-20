create extension if not exists "pgcrypto";

create table if not exists sessions (
    session_id uuid primary key default gen_random_uuid(),
    user_id int not null,
    device_id uuid references devices(device_id) on delete cascade,
    revoked boolean not null default false,
    revoke_at timestamptz,
    created_at timestamptz not null default now(),

    unique (user_id, session_id)
);

create index idx_sessions_user_id on sessions(user_id);
create index idx_sessions_revoked on sessions(revoked);