package storage

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/require"
)

const testUserID = user.ID(7)

var testCursor = &ColorCursor{
	CreatedAt: time.Date(2026, 8, 23, 14, 0, 0, 825023000, time.UTC),
	Hex:       "#ff00aa",
}

// The cursor hex through color.Feel: hue group 11, lightness 660, chroma 276. Spelled out so
// the case pins a value rather than repeating the call it is checking.
const testCursorKey = int32(110660276)

func TestListQueryBuildsOneStatementPerSortAndOrder(t *testing.T) {
	const selectFrom = "SELECT hex, created_at FROM user_colors WHERE user_id = $1"

	cases := []struct {
		name     string
		params   ListColorsParams
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "a zero value is the default first page",
			params:   ListColorsParams{},
			wantSQL:  selectFrom + " ORDER BY created_at DESC, hex DESC LIMIT $2",
			wantArgs: []any{testUserID, 51},
		},
		{
			name:     "created_at asc, first page",
			params:   ListColorsParams{Sort: SortByCreatedAt, Order: OrderAsc, Limit: 10},
			wantSQL:  selectFrom + " ORDER BY created_at ASC, hex ASC LIMIT $2",
			wantArgs: []any{testUserID, 11},
		},
		{
			name:   "created_at desc, after a cursor",
			params: ListColorsParams{Order: OrderDesc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (created_at, hex) < ($3, $4)" +
				" ORDER BY created_at DESC, hex DESC LIMIT $2",
			wantArgs: []any{testUserID, 3, testCursor.CreatedAt, testCursor.Hex},
		},
		{
			name:   "created_at asc, after a cursor, flips the comparison",
			params: ListColorsParams{Order: OrderAsc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (created_at, hex) > ($3, $4)" +
				" ORDER BY created_at ASC, hex ASC LIMIT $2",
			wantArgs: []any{testUserID, 3, testCursor.CreatedAt, testCursor.Hex},
		},
		{
			name:     "hex asc, after a cursor, binds the hex alone",
			params:   ListColorsParams{Sort: SortByHex, Order: OrderAsc, Limit: 2, After: testCursor},
			wantSQL:  selectFrom + " AND hex > $3 ORDER BY hex ASC LIMIT $2",
			wantArgs: []any{testUserID, 3, testCursor.Hex},
		},
		{
			name:     "hex desc, after a cursor",
			params:   ListColorsParams{Sort: SortByHex, Order: OrderDesc, Limit: 2, After: testCursor},
			wantSQL:  selectFrom + " AND hex < $3 ORDER BY hex DESC LIMIT $2",
			wantArgs: []any{testUserID, 3, testCursor.Hex},
		},
		{
			name:     "color asc, first page",
			params:   ListColorsParams{Sort: SortByColor, Order: OrderAsc, Limit: 10},
			wantSQL:  selectFrom + " ORDER BY color_key ASC, hex ASC LIMIT $2",
			wantArgs: []any{testUserID, 11},
		},
		{
			name:   "color asc, after a cursor, binds the key computed from the hex",
			params: ListColorsParams{Sort: SortByColor, Order: OrderAsc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (color_key, hex) > ($3, $4)" +
				" ORDER BY color_key ASC, hex ASC LIMIT $2",
			wantArgs: []any{testUserID, 3, testCursorKey, testCursor.Hex},
		},
		{
			name:   "color desc, after a cursor, flips both columns together",
			params: ListColorsParams{Sort: SortByColor, Order: OrderDesc, Limit: 2, After: testCursor},
			wantSQL: selectFrom + " AND (color_key, hex) < ($3, $4)" +
				" ORDER BY color_key DESC, hex DESC LIMIT $2",
			wantArgs: []any{testUserID, 3, testCursorKey, testCursor.Hex},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query, args, err := c.params.normalized().listQuery(testUserID)

			require.NoError(t, err)
			require.Equal(t, c.wantSQL, flattenSQL(query))
			require.Equal(t, c.wantArgs, args)
			require.Equal(t, len(args), highestPlaceholder(query))
		})
	}
}

func TestListQueryRejectsParamsThatWereNeverParsed(t *testing.T) {
	cases := map[string]ListColorsParams{
		"unknown sort":  {Sort: "name", Order: OrderDesc, Limit: 1},
		"unknown order": {Sort: SortByHex, Order: "sideways", Limit: 1},
	}

	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			query, args, err := params.listQuery(testUserID)

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

			query, _, err := params.normalized().listQuery(testUserID)

			require.NoError(t, err)
			require.NotContains(t, query, string(testCursor.Hex))
			require.NotContains(t, query, "2026")
		})
	}
}

// The template is indented for reading, so a case can state the statement on one line.
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
