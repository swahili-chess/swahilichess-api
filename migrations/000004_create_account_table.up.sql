CREATE TABLE IF NOT EXISTS accounts (
    account_id UUID PRIMARY KEY NOT NULL,
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    firstname text UNIQUE NOT NULL,
    lastname text UNIQUE NOT NULL,
    lichess_username citext UNIQUE NOT NULL,
    chesscom_username  citext UNIQUE NOT NULL,
    phone_number text UNIQUE NOT NULL
);

