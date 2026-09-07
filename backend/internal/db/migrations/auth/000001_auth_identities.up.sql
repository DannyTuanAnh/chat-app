create type auth_identity_status as enum ('active', 'revoked');

create table if not exists auth_identities (
    id serial primary key,
    user_id int not null,
    provider varchar(50) not null,
    provider_user_id varchar(255) not null,
    status auth_identity_status not null default 'active',
    email varchar(255),
    created_at timestamptz not null default now(),
    revoked_at timestamptz,
    
    unique (user_id, provider)
);

create index idx_auth_identities_user_id on auth_identities(user_id);
create unique index uq_auth_identities_provider_user_id on auth_identities(provider, provider_user_id) where status = 'active';