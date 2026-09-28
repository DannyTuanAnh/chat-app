create table if not exists user_status (
    user_id int primary key,
    status smallint not null check(status in (1, 2)),
    updated_at timestamptz not null default now()
);