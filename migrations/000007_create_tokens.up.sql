CREATE TABLE tokens (
	-- Only the SHA-256 of the plaintext (leaked db doesn't have usable tokens)
    hash    bytea PRIMARY KEY,
	-- a deleted user's tokens have nothing left to authenticate
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    expiry  timestamptz NOT NULL
);
