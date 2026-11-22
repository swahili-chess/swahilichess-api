CREATE TABLE lichess_snapshot (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    payload jsonb,
    last_attempt_at timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT NOW(),
    last_attempt_failed boolean NOT NULL DEFAULT false
);
INSERT INTO lichess_snapshot (id) VALUES (true);
