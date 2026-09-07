create extension if not exists "pgcrypto";

create table if not exists messages (
    id bigserial primary key,
    uuid uuid not null unique default gen_random_uuid(),
    conversation_id bigint not null,
    sender_id int not null,
    content text not null,
    sent_at timestamptz not null default now(),
    
    constraint fk_messages_conversation foreign key (conversation_id) references conversations(id) on delete cascade
);

create index idx_messages_conversation_sender_with_id on messages(conversation_id, sender_id) include (id);
create index idx_messages_sender_id on messages(sender_id);
create index idx_messages_conversation on messages(conversation_id, sent_at desc);