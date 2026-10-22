-- Recipes now belong to cookbooks. Existing recipes had no cookbook and were
-- only development data, so they are dropped rather than migrated.
DROP TABLE recipe_tags;
DROP TABLE recipes;

CREATE TABLE cookbooks (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL, -- RFC 3339, UTC
    created_by TEXT NOT NULL  -- username, kept as history even if the user goes
) STRICT;

CREATE TABLE memberships (
    cookbook_id TEXT NOT NULL REFERENCES cookbooks (id) ON DELETE CASCADE,
    user_id     INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK (role IN ('admin', 'editor')),
    joined_at   TEXT NOT NULL, -- RFC 3339, UTC
    PRIMARY KEY (cookbook_id, user_id)
) STRICT;

CREATE INDEX memberships_by_user ON memberships (user_id);

CREATE TABLE recipes (
    id          TEXT PRIMARY KEY,
    cookbook_id TEXT NOT NULL REFERENCES cookbooks (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT '',
    ingredients TEXT NOT NULL, -- JSON array of strings
    steps       TEXT NOT NULL, -- JSON array of strings
    created_at  TEXT NOT NULL, -- RFC 3339, UTC
    created_by  TEXT NOT NULL, -- username, kept as history even if the user goes
    updated_at  TEXT NOT NULL, -- RFC 3339, UTC
    updated_by  TEXT NOT NULL
) STRICT;

CREATE INDEX recipes_by_cookbook ON recipes (cookbook_id);

CREATE TABLE recipe_tags (
    recipe_id TEXT NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    tag       TEXT NOT NULL,
    position  INTEGER NOT NULL, -- keeps the order the user gave
    PRIMARY KEY (recipe_id, tag)
) STRICT;

CREATE INDEX recipe_tags_by_tag ON recipe_tags (tag);
