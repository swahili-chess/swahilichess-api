CREATE EXTENSION citext;

CREATE TABLE IF NOT EXISTS users (
    uuid UUID PRIMARY KEY NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    username text UNIQUE NOT NULL,
    email citext UNIQUE NOT NULL,
    firstname text,
    lastname text,
    password_hash bytea NOT NULL,
    activated bool NOT NULL,
    version integer NOT NULL DEFAULT 1
);


