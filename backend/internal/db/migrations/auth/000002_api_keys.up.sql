create table if not exists api_keys (
    id serial primary key,
    key_hash varchar(255) not null unique,
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    revoked_at timestamptz
);
