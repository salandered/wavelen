CREATE TABLE user_colors (
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hex        text NOT NULL CHECK (hex ~ '^#[0-9a-f]{6}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, hex)
);
