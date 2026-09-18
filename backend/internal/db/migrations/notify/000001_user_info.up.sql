create table if not exists user_info (
    user_id int primary key,
    display_name varchar(50) not null,
    device_uuid uuid not null unique,
    push_token text not null  
);