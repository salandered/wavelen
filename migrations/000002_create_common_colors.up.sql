CREATE TABLE common_colors (
	-- '~' - case-sensitive regex; six lowercase hex digits
    hex  text PRIMARY KEY CHECK (hex ~ '^#[0-9a-f]{6}$'),
    name text NOT NULL UNIQUE
);
