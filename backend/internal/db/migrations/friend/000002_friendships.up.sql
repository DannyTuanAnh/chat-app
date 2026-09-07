create table if not exists friendships (
    user1_id int not null,
    user2_id int not null,
    established_at timestamptz not null default now(),

    primary key (user1_id, user2_id),

    constraint check_user_order check (user1_id < user2_id)
);

create index idx_friendships_user2_id on friendships(user2_id);
