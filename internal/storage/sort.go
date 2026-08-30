package storage

import (
	"errors"
	"fmt"
)

type SortOrder string

const (
	OrderAsc  SortOrder = "asc"
	OrderDesc SortOrder = "desc"
)

var ErrInvalidSortOrder = errors.New("invalid order")

func ParseSortOrder(s string) (SortOrder, error) {
	switch order := SortOrder(s); order {
	case OrderAsc, OrderDesc:
		return order, nil
	}
	return "", fmt.Errorf(
		"%w: want %q or %q, got %q",
		ErrInvalidSortOrder, OrderAsc, OrderDesc, s,
	)
}
