create table if not exists user_info (
    user_id int not null,
    device_uuid uuid not null unique,
    push_token text not null,

    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),

    primary key (user_id, device_uuid)
);