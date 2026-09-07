-- hex is the tiebreak (ordering by created_at is a total order)
-- Sorting by hex does not need an index: PK is already a btree on (user_id, hex).
CREATE INDEX user_colors_user_id_created_at_hex_idx ON user_colors (user_id, created_at, hex);
