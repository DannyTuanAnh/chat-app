create type member_role as enum ('admin', 'member');

create table if not exists conversation_members(
    conversation_id bigint not null,
    user_id int not null,
    role member_role not null default 'member',
    joined_at timestamptz not null default now(),

    constraint fk_conversation_members_conversations foreign key (conversation_id) references conversations(id) on delete cascade,
    constraint unique_conversation_member unique (conversation_id, user_id)
);

create index idx_conversation_members_conversation_id on conversation_members(conversation_id);
create index idx_conversation_members_user_id on conversation_members(user_id);

