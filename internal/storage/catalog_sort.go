package storage

import (
	"errors"
	"fmt"
)

// A column. The shared palette is ordered by it
type CatalogSort string

const (
	CatalogSortByName  CatalogSort = "name"
	CatalogSortByHex   CatalogSort = "hex"
	CatalogSortByColor CatalogSort = "color"
)

const (
	DefaultCatalogSort  = CatalogSortByName
	DefaultCatalogOrder = OrderAsc
)

var ErrInvalidCatalogSort = errors.New("invalid sort")

func ParseCatalogSort(s string) (CatalogSort, error) {
	switch sort := CatalogSort(s); sort {
	case CatalogSortByName, CatalogSortByHex, CatalogSortByColor:
		return sort, nil
	}
	return "", fmt.Errorf("%w: want %q, %q or %q, got %q",
		ErrInvalidCatalogSort, CatalogSortByName, CatalogSortByHex, CatalogSortByColor, s)
}

// The palette is 100 fixed rows (no limit or cursor)
type ListCommonColorsParams struct {
	Sort  CatalogSort
	Order SortOrder
}

// A zero value means the defaults.
func (p ListCommonColorsParams) normalized() ListCommonColorsParams {
	if p.Sort == "" {
		p.Sort = DefaultCatalogSort
	}
	if p.Order == "" {
		p.Order = DefaultCatalogOrder
	}
	return p
}

// name and hex are unique - a total order
// color_key is not unique: two similar shades can round to the same key, so it is tied by hex
func (p ListCommonColorsParams) orderBy() (string, error) {
	var dir string
	switch p.Order {
	case OrderAsc:
		dir = "ASC"
	case OrderDesc:
		dir = "DESC"
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidSortOrder, p.Order)
	}

	switch p.Sort {
	case CatalogSortByName:
		return "name " + dir, nil
	case CatalogSortByHex:
		return "hex " + dir, nil
	case CatalogSortByColor:
		// both columns take the same direction, same rule as the keyset predicate
		return "color_key " + dir + ", hex " + dir, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidCatalogSort, p.Sort)
}
