CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE, -- normalized to lowercase
    password_hash TEXT NOT NULL,        -- argon2id, PHC string format
    created_at    TEXT NOT NULL         -- RFC 3339, UTC
) STRICT;

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY, -- SHA-256 of the cookie value; the token itself is never stored
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,    -- RFC 3339, UTC
    expires_at TEXT NOT NULL     -- RFC 3339, UTC; pushed back while the session is used
) STRICT;

CREATE INDEX sessions_by_user ON sessions (user_id);
