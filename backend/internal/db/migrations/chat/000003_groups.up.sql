create table if not exists groups (
    conversation_id uuid primary key,
    name varchar(255) not null,
    avatar_url text,
    created_at timestamptz not null default now(),

    constraint fk_group_conversation foreign key (conversation_id) references conversations(id) on delete cascade
);
