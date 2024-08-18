CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION citext;

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY NOT NULL,
    username citext UNIQUE NOT NULL,
    email citext UNIQUE NOT NULL,
    full_name text NOT NULL,
    lichess_username citext UNIQUE NOT NULL,
    chesscom_username  citext UNIQUE,
    phone_number text UNIQUE NOT NULL,
    password_hash bytea NOT NULL,
    activated bool NOT NULL,
    photos text NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tokens (
    hash bytea PRIMARY KEY,
    uuid UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    expiry timestamp(0) with time zone NOT NULL,
    scope text NOT NULL
);

