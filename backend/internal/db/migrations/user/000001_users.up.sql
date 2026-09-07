create extension if not exists "pgcrypto";

create table if not exists users (
    user_id serial primary key,
    uuid uuid not null unique default gen_random_uuid(),
    display_name varchar(50) not null,
    created_at timestamptz not null default now(),
    is_active boolean not null default true,
    disable_at timestamptz default null
);

create index idx_users_uuid on users(uuid);