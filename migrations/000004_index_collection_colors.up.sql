-- hex is the tiebreak
CREATE INDEX collection_colors_created_at_hex_idx
    ON collection_colors (collection_id, created_at, hex);

-- hex is the tiebreak: two near-identical shades can round to the same color_key.
CREATE INDEX collection_colors_color_key_hex_idx
    ON collection_colors (collection_id, color_key, hex);

-- Sorting by hex does not need an index: PK is already a btree on (collection_id, hex).