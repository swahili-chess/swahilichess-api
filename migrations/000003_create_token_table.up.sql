CREATE TABLE IF NOT EXISTS tokens (
hash bytea PRIMARY KEY,
uuid UUID NOT NULL REFERENCES users ON DELETE CASCADE,
expiry timestamp(0) with time zone NOT NULL,
scope text NOT NULL
);