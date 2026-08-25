package storage

import (
	"errors"
	"fmt"
	"time"

	"github.com/salandered/wavelen/internal/color"
)

// ColorSort is a column a colors listing is ordered by.
type ColorSort string

const (
	SortByCreatedAt ColorSort = "created_at"
	SortByHex       ColorSort = "hex"
	SortByColor     ColorSort = "color"
)

const (
	DefaultColorSort  = SortByCreatedAt
	DefaultColorOrder = OrderDesc
	DefaultColorLimit = 50
	MaxColorLimit     = 100
)

var ErrInvalidColorSort = errors.New("invalid sort")

func ParseColorSort(s string) (ColorSort, error) {
	switch sort := ColorSort(s); sort {
	case SortByCreatedAt, SortByHex, SortByColor:
		return sort, nil
	}
	return "", fmt.Errorf("%w: want %q, %q or %q, got %q",
		ErrInvalidColorSort, SortByCreatedAt, SortByHex, SortByColor, s)
}

// ColorCursor is the ordering key of the last row of the previous page.
type ColorCursor struct {
	CreatedAt time.Time
	Hex       color.Hex
}

type ListColorsParams struct {
	Sort  ColorSort
	Order SortOrder
	After *ColorCursor // nil on the first page
	Limit int
}

type ColorPage struct {
	Colors  []color.Color
	HasMore bool
}

// A zero value means the defaults.
func (p ListColorsParams) normalized() ListColorsParams {
	if p.Sort == "" {
		p.Sort = DefaultColorSort
	}
	if p.Order == "" {
		p.Order = DefaultColorOrder
	}
	switch {
	case p.Limit <= 0:
		p.Limit = DefaultColorLimit
	case p.Limit > MaxColorLimit:
		p.Limit = MaxColorLimit
	}
	return p
}

// One switch picks the ORDER BY columns, the keyset comparison and the cursor values to bind
func (p ListColorsParams) keyset() (orderBy, predicate string, cursorArgs []any, err error) {
	switch p.Sort {
	case SortByCreatedAt:
		if p.After != nil {
			cursorArgs = []any{p.After.CreatedAt, p.After.Hex}
		}
		switch p.Order {
		case OrderDesc:
			orderBy, predicate = "created_at DESC, hex DESC", "(created_at, hex) < ($3, $4)"
		case OrderAsc:
			orderBy, predicate = "created_at ASC, hex ASC", "(created_at, hex) > ($3, $4)"
		default:
			return "", "", nil, fmt.Errorf("%w: %q", ErrInvalidSortOrder, p.Order)
		}
	case SortByHex:
		// hex is the second half of the primary key, so it is already a total order
		if p.After != nil {
			cursorArgs = []any{p.After.Hex}
		}
		switch p.Order {
		case OrderAsc:
			orderBy, predicate = "hex ASC", "hex > $3"
		case OrderDesc:
			orderBy, predicate = "hex DESC", "hex < $3"
		default:
			return "", "", nil, fmt.Errorf("%w: %q", ErrInvalidSortOrder, p.Order)
		}
	case SortByColor:
		// The key is derived from the hex, so the cursor carries the hex alone and the boundary
		// key is recomputed here rather than stored in the token.
		if p.After != nil {
			cursorArgs = []any{color.Feel(p.After.Hex), p.After.Hex}
		}
		switch p.Order {
		case OrderAsc:
			orderBy, predicate = "color_key ASC, hex ASC", "(color_key, hex) > ($3, $4)"
		case OrderDesc:
			orderBy, predicate = "color_key DESC, hex DESC", "(color_key, hex) < ($3, $4)"
		default:
			return "", "", nil, fmt.Errorf("%w: %q", ErrInvalidSortOrder, p.Order)
		}
	default:
		return "", "", nil, fmt.Errorf("%w: %q", ErrInvalidColorSort, p.Sort)
	}
	return orderBy, predicate, cursorArgs, nil
}
