create type conversation_type as enum ('private', 'group');

create table if not exists conversations (
    id uuid primary key default gen_random_uuid(),
    type conversation_type not null,
    created_at timestamptz not null default now()
);

create index idx_conversations_type on conversations(type);

