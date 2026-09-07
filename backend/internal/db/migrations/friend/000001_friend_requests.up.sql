create table if not exists friend_requests (
    request_id serial primary key,
    sender_id int not null,
    receiver_id int not null,
    is_accepted boolean not null default false,
    send_at timestamptz not null default now(),

    constraint check_no_self_request check (sender_id <> receiver_id)
);

create unique index unique_friend_request on friend_requests(least(sender_id, receiver_id), greatest(sender_id, receiver_id));
create index idx_friend_requests_pending_sender_receiver on friend_requests(sender_id, receiver_id) where is_accepted = false;
create index idx_friend_requests_receiver_id on friend_requests(receiver_id);


