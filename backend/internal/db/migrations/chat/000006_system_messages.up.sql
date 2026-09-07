create extension if not exists "pgcrypto";

create type system_event_type as enum (
    'member_joined',
    'member_left',
    'member_removed',
    'member_added',
    'chat_created',
    'group_created',
    'group_name_changed',
    'group_avatar_changed',
    'member_promoted',
    'member_demoted'
);

create table if not exists system_messages (
    id bigserial primary key,
    uuid uuid not null unique default gen_random_uuid(),
    conversation_id bigint not null,
    event_type system_event_type not null,
    actor_id int,
    target_id int,
    content text,
    created_at timestamptz not null default now(),
    
    constraint fk_system_messages_conversation foreign key (conversation_id) references conversations(id) on delete cascade
);

create index idx_system_messages_conversation on system_messages(conversation_id, created_at desc);
create index idx_system_messages_event_type on system_messages(event_type);

