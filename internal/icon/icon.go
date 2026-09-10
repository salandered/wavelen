/*
The glyphs live in web/index.html as <symbol id="i-slug">, copied from Lucide 1.43.0 (ISC).
*/
package icon

import (
	"errors"
	"fmt"
	"slices"
)

type Slug string

const Default Slug = "square"

// Sorted and unique (covered by tests).
var slugs = [...]Slug{
	"anchor",
	"brush",
	"camera",
	"cat",
	"cloud",
	"coffee",
	"crown",
	"droplet",
	"fish",
	"flower",
	"gem",
	"heart",
	"image",
	"layers",
	"leaf",
	"moon",
	"mountain",
	"music",
	"palette",
	"pipette",
	"snowflake",
	"square",
	"star",
	"sun",
	"tag",
}

var ErrUnknownIcon = errors.New("unknown icon")

// ParseSlug creates a Slug out of s. Validates against the enum [slugs].
func ParseSlug(s string) (Slug, error) {
	slug := Slug(s)
	if _, found := slices.BinarySearch(slugs[:], Slug(s)); !found {
		return "", fmt.Errorf("%w: %q", ErrUnknownIcon, s)
	}
	return slug, nil
}
