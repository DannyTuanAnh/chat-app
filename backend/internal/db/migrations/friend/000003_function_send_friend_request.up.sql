create or replace function send_friend_request(p_sender_id int, p_receiver_id int)
returns table (
    status boolean,
    message text
)
language plpgsql
as $$
declare
    v_existing_sender int;
begin

    status := false;

    if p_sender_id = p_receiver_id then
        message := 'You cannot send a friend request to yourself';
        return next;
        return;
    end if;

    perform pg_advisory_xact_lock(least(p_sender_id, p_receiver_id), greatest(p_sender_id, p_receiver_id));

    -- 1. check friendship
    if exists (
        select 1
        from friendships
        where user1_id = least(p_sender_id, p_receiver_id) and user2_id = greatest(p_sender_id, p_receiver_id)
    ) then
        message := 'already_friends';
        return next;
        return;
    end if;

    -- 2. create friend request (if not exists)
    insert into friend_requests (sender_id, receiver_id)
    values (p_sender_id, p_receiver_id)
    on conflict do nothing;

    -- if failed to insert -> check if there's an existing pending request 
    if not found then

        select sender_id into v_existing_sender 
        from friend_requests 
        where sender_id = p_receiver_id and receiver_id = p_sender_id and is_accepted = false
        limit 1;
        if v_existing_sender = p_receiver_id then
            message := 'This person has already sent you a friend request';
        else
            message := 'Friend request already sent';
        end if;

        return next;
        return;
    end if;

    status := true;
    message := 'Friend request sent successfully';

    return next;
end;
$$;