-- hex is the tiebreak: color_key is not unique, two near-identical shades can round to the same key.
-- Equality column first, same reason as 000005 - one user's rows are then a contiguous slice.
CREATE INDEX user_colors_user_id_color_key_hex_idx ON user_colors (user_id, color_key, hex);
