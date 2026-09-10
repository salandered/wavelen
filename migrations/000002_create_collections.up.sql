CREATE TABLE collections (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id     bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	-- free form display name, not unique
    name        text NOT NULL,
	-- an icon slug tinted with accent
    icon_slug   text NOT NULL DEFAULT 'square',
    icon_accent text NOT NULL DEFAULT '#808080',
    is_default  boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Only one default per user.
-- The index the default lookup reads.
CREATE UNIQUE INDEX collections_one_default_per_user ON collections (user_id) WHERE is_default;
