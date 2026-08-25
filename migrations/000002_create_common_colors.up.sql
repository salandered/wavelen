CREATE TABLE common_colors (
	-- '~' - case-sensitive regex; six lowercase hex digits
    hex  text PRIMARY KEY CHECK (hex ~ '^#[0-9a-f]{6}$'),
    name text NOT NULL UNIQUE,
    -- the perceptual ordering key, color.SortKey(hex). Seeded with the rows, never recomputed here
    color_key integer NOT NULL
);
