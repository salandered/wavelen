CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id         bigserial PRIMARY KEY,
	-- the login identifier. case insensitive comparisons
    nickname   citext NOT NULL UNIQUE,
	-- bytea - binary string. Stores the bcrypt hash
    password_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
