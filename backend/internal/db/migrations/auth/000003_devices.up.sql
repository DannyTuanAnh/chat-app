create table if not exists devices (
    device_id uuid primary key,
    user_id int not null,
    created_at timestamptz default now(),
    last_seen_at timestamptz,
    revoked_at timestamptz
);