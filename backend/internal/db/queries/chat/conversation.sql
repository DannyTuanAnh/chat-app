-- -- name: GetAllConversations :many
-- with user_conversations as (
--     select conversation_id
--     from conversation_members
--     where user_id = $1
-- ),

-- all_timeline as (
--     -- User messages
--     select 
--         m.id,
--         m.conversation_id,
--         'user_message' as message_type,
--         m.content as last_message,
--         m.sent_at as last_message_time,
--         m.sender_id,
--         coalesce(p.name, u.display_name) as sender_name,
--         null::bigint as actor_id,
--         null::bigint as target_id,
--         null::text as event_type,
--         null::text as actor_name,
--         null::text as target_name
--     from messages m
--     join users u on m.sender_id = u.user_id
--     left join profiles p on m.sender_id = p.user_id
    
--     union all
    
--     -- System events with actor/target names
--     select 
--         sm.id,
--         sm.conversation_id,
--         'system_event' as message_type,
--         sm.content as last_message,
--         sm.created_at as last_message_time,
--         null::bigint as sender_id,
--         null::text as sender_name,
--         sm.actor_id,
--         sm.target_id,
--         sm.event_type::text as event_type,
--         coalesce(actor_p.name, actor_u.display_name) as actor_name,
--         coalesce(target_p.name, target_u.display_name) as target_name
--     from system_messages sm
--     left join users actor_u on sm.actor_id = actor_u.user_id
--     left join users target_u on sm.target_id = target_u.user_id
--     left join profiles actor_p on sm.actor_id = actor_p.user_id
--     left join profiles target_p on sm.target_id = target_p.user_id
-- ),

-- latest_messages as (
--     select distinct on (conversation_id)
--         id,
--         conversation_id,
--         message_type,
--         last_message,
--         last_message_time,
--         sender_id,
--         sender_name,
--         actor_id,
--         target_id,
--         event_type,
--         actor_name,
--         target_name
--     from all_timeline
--     order by conversation_id, last_message_time desc
-- )

-- select
--     c.id as conversation_id,
--     c.type as conversation_type,
    
--     -- Name and avatar based on conversation type
--     case 
--         when c.type = 'private' then
--             (select p.name
--              from conversation_members cm
--              join profiles p on cm.user_id = p.user_id
--              where cm.conversation_id = c.id 
--                and cm.user_id <> $1
--              limit 1)
--         else
--             (select g.name
--              from groups g
--              where g.conversation_id = c.id)
--     end as conversation_name,
    
--     case 
--         when c.type = 'private' then
--             (select p.avatar_url
--              from conversation_members cm
--              join profiles p on cm.user_id = p.user_id
--              where cm.conversation_id = c.id 
--                and cm.user_id <> $1
--              limit 1)
--         else
--             (select g.avatar_url
--              from groups g
--              where g.conversation_id = c.id)
--     end as avatar_url,
    
--     -- Last message info
--     lm.message_type,
--     lm.last_message,
--     lm.last_message_time,
--     lm.sender_id,
--     lm.sender_name,
--     lm.actor_id,
--     lm.target_id,
--     lm.event_type,
--     lm.actor_name,
--     lm.target_name,

--     (mr.message_id is not null) as is_read

-- from user_conversations uc
-- join conversations c on uc.conversation_id = c.id
-- left join latest_messages lm on c.id = lm.conversation_id
-- left join message_reads mr on lm.id = mr.message_id and mr.user_id = $1
-- order by coalesce(lm.last_message_time, c.created_at) desc;

-- -- name: GetMessagesByConversationId :many
-- with combined_timeline as (

--     select 
--         m.id,
--         'user_message' as message_type,
--         m.sender_id,
--         null::bigint as actor_id,
--         null::bigint as target_id,
--         null::text as event_type,
--         m.conversation_id,
--         m.content,
--         m.sent_at as created_at,
--         coalesce(p.name, u.display_name) as sender_name,
--         null::varchar(50) as target_name,
--         p.avatar_url as sender_avatar,
--         case
--             when m.sender_id = $2 then 'sent'
--             else 'received'
--         end as message_direction
--     from messages m
--     join users u on u.user_id = m.sender_id
--     left join profiles p on p.user_id = m.sender_id
--     where m.conversation_id = $1

--     union all

--     select 
--         sm.id,
--         'system_event' as message_type,
--         null::bigint as sender_id,
--         sm.actor_id,
--         sm.target_id,
--         sm.event_type::text as event_type,
--         sm.conversation_id,
--         sm.content,
--         sm.created_at,
--         coalesce(actor_p.name, actor_u.display_name) as sender_name,
--         coalesce(target_p.name, target_u.display_name) as target_name,
--         null::text as sender_avatar,
--         null::text as message_direction
--     from system_messages sm
--     left join users actor_u on sm.actor_id = actor_u.user_id
--     left join users target_u on sm.target_id = target_u.user_id
--     left join profiles actor_p on sm.actor_id = actor_p.user_id
--     left join profiles target_p on sm.target_id = target_p.user_id
--     where sm.conversation_id = $1
-- )
-- select * from combined_timeline
-- where ($3::bigint is null or id < $3)
-- order by id desc 
-- limit $4;


-- name: MarkMessagesAsRead :exec
insert into message_reads (message_id, user_id, read_at)
select m.id, $2, now()
from messages m
left join message_reads mr on m.id = mr.message_id and mr.user_id = $2
where m.conversation_id = $1
  and m.sender_id <> $2
  and mr.message_id is null;

-- name: CreateConversation :one
insert into conversations(type)
values($1)
returning id;

-- name: AddMembersToConversation :execresult
insert into conversation_members (conversation_id, user_id)
select $1, unnest(sqlc.arg(user_ids)::int[]);

-- name: GetConversationMembers :many
select cm.user_id from conversation_members cm
where cm.conversation_id = $1
  and exists (select 1 from conversation_members cm where cm.conversation_id = $1 and cm.user_id = $2);

-- name: CreateMessage :one
with members as (
  select user_id from conversation_members
  where conversation_id = $1
)
insert into messages (conversation_id, sender_id, content)
select $1, $2, $3
from conversation_members 
where conversation_id = $1 and user_id = $2
returning id, conversation_id, sender_id, content, sent_at, 
  (select json_agg(json_build_object('user_id', user_id)) from members) as members;

-- name: CreateSystemMessage :one
insert into system_messages (conversation_id, event_type, actor_id, target_id, content)
values ($1, $2, $3, $4, $5)
returning id, conversation_id, event_type, actor_id, target_id, content, created_at;

-- -- name: SearchConversationByName :many
-- with user_convs as (
--     -- 1. Get my conversations
--     select conversation_id 
--     from conversation_members cm
--     where cm.user_id = $1
-- ),
-- conv_details as (
--     -- 2. Get conversation details (name/avatar) based on type
--     select 
--         c.id as conversation_id,
--         c.type,
--         case 
--             when c.type = 'group' then g.name
--             else coalesce(p.name, u.display_name)
--         end as conv_name,
--         case 
--             when c.type = 'group' then g.avatar_url
--             else p.avatar_url
--         end as conv_avatar
--     from conversations c
--     join user_convs uc on c.id = uc.conversation_id
--     left join groups g on c.id = g.conversation_id and c.type = 'group'
--     -- For private chats, get the other member's profile info
--     left join conversation_members other_cm on c.id = other_cm.conversation_id 
--         and c.type = 'private' and other_cm.user_id <> $1
--     left join users u on other_cm.user_id = u.user_id
--     left join profiles p on u.user_id = p.user_id
-- )
-- select 
--     conversation_id,
--     type as conversation_type,
--     conv_name as conversation_name,
--     conv_avatar as avatar_url
-- from conv_details
-- where conv_name ilike '%' || $2 || '%'
-- order by conv_name;

