-- One-time links for choosing a password. Accounts are created without a
-- password (password_hash = ''), and admins reset passwords the same way.
CREATE TABLE setup_tokens (
    token_hash TEXT PRIMARY KEY, -- SHA-256 of the token in the link; the token itself is never stored
    user_id    INTEGER NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE, -- one live link per user
    created_at TEXT NOT NULL,    -- RFC 3339, UTC
    expires_at TEXT NOT NULL     -- RFC 3339, UTC
) STRICT;
