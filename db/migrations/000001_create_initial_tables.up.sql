CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION citext;

CREATE TABLE IF NOT EXISTS users (
    id uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
    username citext UNIQUE NOT NULL,
    full_name text NOT NULL,
    lichess_username citext UNIQUE,
    chesscom_username  citext UNIQUE,
    phone_number text UNIQUE NOT NULL,
    password_hash bytea NOT NULL,
    activated bool NOT NULL,
    photo text NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tokens (
    hash bytea PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    expiry timestamp(0) with time zone NOT NULL,
    scope text NOT NULL
);

-- Below  table used by the chessbot on nyumbani mates team, We only read here
CREATE TABLE IF NOT EXISTS lichess (
    id SERIAL PRIMARY KEY,
    lichess_id TEXT UNIQUE NOT NULL,
    username TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP(0) NOT NULL DEFAULT NOW()
);
