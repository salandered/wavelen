CREATE TABLE collections (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	-- free form display name
    name       text NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Only one default per user.
-- The index the default lookup reads.
CREATE UNIQUE INDEX collections_one_default_per_user ON collections (user_id) WHERE is_default;
