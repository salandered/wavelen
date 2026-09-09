package storage

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/stretchr/testify/require"
)

var testCollectionID = collection.ID(
	uuid.MustParse("01999999-7777-7777-8888-999999999999"))

var testCursor = &ColorCursor{
	CreatedAt: time.Date(2026, 8, 23, 14, 0, 0, 825023000, time.UTC),
	Hex:       "#ff00aa",
}

// The cursor hex through color.Feel: hue group 11, lightness 660, chroma 276.
// Could be written as color.Feel(testCursor.Hex) but it's better for test to fail if the formula changes
const testCursorKey = int32(110660276)

func TestListQueryActualSQLString(t *testing.T) {
	const selectFrom = "SELECT hex, created_at FROM collection_colors WHERE collection_id = $1"

	cases := []struct {
		name     string
		params   ListColorsParams
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "zero value is the default first page",
			params:   ListColorsParams{},
			wantSQL:  selectFrom + " ORDER BY created_at DESC, hex DESC LIMIT $2",
			wantArgs: []any{testCollectionID, 51},
		},
		{
			name:     "created_at asc, first page",
			params:   ListColorsParams{Sort: SortByCreatedAt, Order: OrderAsc, Limit: 10},
			wantSQL:  selectFrom + " ORDER BY created_at ASC, hex ASC LIMIT $2",
			wantArgs: []any{testCollectionID, 11},
		},
		{
			name:   "created_at desc, after cursor",
			params: ListColorsParams{Order: OrderDesc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (created_at, hex) < ($3, $4)" +
				" ORDER BY created_at DESC, hex DESC LIMIT $2",
			wantArgs: []any{testCollectionID, 3, testCursor.CreatedAt, testCursor.Hex},
		},
		{
			name:   "created_at asc, after cursor",
			params: ListColorsParams{Order: OrderAsc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (created_at, hex) > ($3, $4)" +
				" ORDER BY created_at ASC, hex ASC LIMIT $2",
			wantArgs: []any{testCollectionID, 3, testCursor.CreatedAt, testCursor.Hex},
		},
		{
			name:     "hex asc, after cursor, binds only hex",
			params:   ListColorsParams{Sort: SortByHex, Order: OrderAsc, Limit: 2, After: testCursor},
			wantSQL:  selectFrom + " AND hex > $3 ORDER BY hex ASC LIMIT $2",
			wantArgs: []any{testCollectionID, 3, testCursor.Hex},
		},
		{
			name:     "hex desc, after cursor",
			params:   ListColorsParams{Sort: SortByHex, Order: OrderDesc, Limit: 2, After: testCursor},
			wantSQL:  selectFrom + " AND hex < $3 ORDER BY hex DESC LIMIT $2",
			wantArgs: []any{testCollectionID, 3, testCursor.Hex},
		},
		{
			name:     "color asc, first page",
			params:   ListColorsParams{Sort: SortByColor, Order: OrderAsc, Limit: 10},
			wantSQL:  selectFrom + " ORDER BY color_key ASC, hex ASC LIMIT $2",
			wantArgs: []any{testCollectionID, 11},
		},
		{
			name:   "color asc, after cursor, binds the key computed from hex",
			params: ListColorsParams{Sort: SortByColor, Order: OrderAsc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (color_key, hex) > ($3, $4)" +
				" ORDER BY color_key ASC, hex ASC LIMIT $2",
			wantArgs: []any{testCollectionID, 3, testCursorKey, testCursor.Hex},
		},
		{
			name:   "color desc, after cursor",
			params: ListColorsParams{Sort: SortByColor, Order: OrderDesc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (color_key, hex) < ($3, $4)" +
				" ORDER BY color_key DESC, hex DESC LIMIT $2",
			wantArgs: []any{testCollectionID, 3, testCursorKey, testCursor.Hex},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query, args, err := c.params.normalized().listQuery(testCollectionID)

			require.NoError(t, err)
			require.Equal(t, c.wantSQL, flattenSQL(query))
			require.Equal(t, c.wantArgs, args)
			require.Equal(t, len(args), highestPlaceholder(query))
		})
	}
}

func TestListQueryRejectsUnknownListColorsParams(t *testing.T) {
	cases := map[string]ListColorsParams{
		"unknown sort":  {Sort: "name", Order: OrderDesc, Limit: 1},
		"unknown order": {Sort: SortByHex, Order: "sideways", Limit: 1},
	}

	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			query, args, err := params.listQuery(testCollectionID)

			require.Error(t, err)
			require.Empty(t, query) // nothing half-built reaches the pool
			require.Nil(t, args)
		})
	}
}

func TestListQueryNeverInterpolatesACursorValue(t *testing.T) {
	for _, sort := range []ColorSort{SortByCreatedAt, SortByHex, SortByColor} {
		t.Run(string(sort), func(t *testing.T) {
			params := ListColorsParams{Sort: sort, Limit: 2, After: testCursor}

			query, _, err := params.normalized().listQuery(testCollectionID)

			require.NoError(t, err)
			require.NotContains(t, query, string(testCursor.Hex))
			require.NotContains(t, query, "2026")
		})
	}
}

func flattenSQL(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

// Every $n up to the count of args must be present, and no more.
func highestPlaceholder(query string) int {
	for n := 1; ; n++ {
		if !strings.Contains(query, "$"+strconv.Itoa(n)) {
			return n - 1
		}
	}
}
