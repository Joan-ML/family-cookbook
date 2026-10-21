CREATE TABLE recipes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT '',
    ingredients TEXT NOT NULL, -- JSON array of strings
    steps       TEXT NOT NULL, -- JSON array of strings
    created_at  TEXT NOT NULL, -- RFC 3339, UTC
    created_by  TEXT NOT NULL,
    updated_at  TEXT NOT NULL, -- RFC 3339, UTC
    updated_by  TEXT NOT NULL
) STRICT;

CREATE TABLE recipe_tags (
    recipe_id TEXT NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    tag       TEXT NOT NULL,
    position  INTEGER NOT NULL, -- keeps the order the user gave
    PRIMARY KEY (recipe_id, tag)
) STRICT;

CREATE INDEX recipe_tags_by_tag ON recipe_tags (tag);
