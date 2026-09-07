create table if not exists profiles(
    user_id int primary key,
    name varchar(50) not null,
    email varchar(255),
    phone varchar(20) unique,
    birthday date,
    avatar_url text,
    avatar_version int not null default 0,
    updated_at timestamptz not null default now(),
    
    constraint fk_user_profile foreign key (user_id) references users(user_id) on delete cascade
);

