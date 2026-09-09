CREATE TABLE collection_colors (
    collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    hex           text NOT NULL CHECK (hex ~ '^#[0-9a-f]{6}$'),
    created_at    timestamptz NOT NULL DEFAULT now(),
    -- the perceptual ordering key
    color_key     integer NOT NULL,
    PRIMARY KEY (collection_id, hex)
);
