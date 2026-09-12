//go:build e2e

package e2e

import (
	"net/http"
	"slices"
	"strings"
	"time"
)

type collectionJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Accent    string `json:"accent"`
	IsDefault bool   `json:"is_default"`
}

type harmonyJSON struct {
	Hex     string   `json:"hex"`
	Harmony string   `json:"harmony"`
	Colors  []string `json:"colors"`
}

type commonColorJSON struct {
	Hex  string `json:"hex"`
	Name string `json:"name"`
}

// not using [E2ESuite.signUpAndLogIn] on purpose
func (s *E2ESuite) TestAuthLifecycleFromSignupToDeletion() {
	s.nickname = uniqueNickname()

	s.step("sign up", func() {
		resp := s.post("/api/v1/users", map[string]string{
			"nickname": s.nickname,
			"password": testPassword,
		})
		s.requireStatus(resp, http.StatusCreated)
	})

	s.step("log in", func() {
		resp := s.post("/api/v1/tokens", map[string]string{
			"nickname": s.nickname,
			"password": testPassword,
		})
		s.requireStatus(resp, http.StatusCreated)

		var login struct {
			Token  string    `json:"token"`
			Expiry time.Time `json:"expiry"`
		}
		s.jsonDecode(resp, &login)
		s.Require().NotEmpty(login.Token)
		s.Require().True(
			login.Expiry.After(time.Now()),
			"token expiry %s is in the past", login.Expiry)
		s.token = login.Token
	})

	s.step("token identifies its account", func() {
		resp := s.get("/api/v1/me")
		s.requireStatus(resp, http.StatusOK)

		var me struct {
			User struct {
				Nickname string `json:"nickname"`
			} `json:"user"`
		}
		s.jsonDecode(resp, &me)
		s.Require().Equal(s.nickname, me.User.Nickname)
	})

	s.step("log out", func() {
		resp := s.del("/api/v1/tokens")
		s.requireStatus(resp, http.StatusNoContent)
	})

	s.step("logged out token is rejected", func() {
		resp := s.get("/api/v1/me")
		s.requireStatus(resp, http.StatusUnauthorized)
	})

	s.step("log in again", func() {
		resp := s.post("/api/v1/tokens", map[string]string{
			"nickname": s.nickname,
			"password": testPassword,
		})
		s.requireStatus(resp, http.StatusCreated)

		var login struct {
			Token string `json:"token"`
		}
		s.jsonDecode(resp, &login)
		s.Require().NotEmpty(login.Token)
		s.token = login.Token
	})

	s.step("delete account", func() {
		resp := s.del("/api/v1/me")
		s.requireStatus(resp, http.StatusNoContent)
	})

	s.step("token died with the account", func() {
		resp := s.get("/api/v1/me")
		s.requireStatus(resp, http.StatusUnauthorized)
	})
}

func (s *E2ESuite) TestCollectionAndColorLifecycle() {
	s.signUpAndLogIn()

	s.step("sign up created the default collection", func() {
		resp := s.get("/api/v1/me/collections")
		s.requireStatus(resp, http.StatusOK)

		var list struct {
			Collections []collectionJSON `json:"collections"`
		}
		s.jsonDecode(resp, &list)
		s.Require().Len(list.Collections, 1)
		s.Require().True(list.Collections[0].IsDefault)
	})

	s.step("add collection", func() {
		resp := s.post("/api/v1/me/collections", map[string]string{"name": "Sun"})
		s.requireStatus(resp, http.StatusCreated)

		var created struct {
			Collection collectionJSON `json:"collection"`
		}
		s.jsonDecode(resp, &created)
		s.Require().Equal("Sun", created.Collection.Name)
		s.Require().False(created.Collection.IsDefault)
		s.Require().NotEmpty(created.Collection.ID)
		s.collectionID = created.Collection.ID
	})

	s.step("read collection by id", func() {
		resp := s.get(s.collectionPath())
		s.requireStatus(resp, http.StatusOK)

		var got struct {
			Collection collectionJSON `json:"collection"`
		}
		s.jsonDecode(resp, &got)
		s.Require().Equal(s.collectionID, got.Collection.ID)
		s.Require().Equal("Sun", got.Collection.Name)
	})

	s.step("add color", func() {
		resp := s.post(s.colorsPath(), map[string]string{"hex": "FF00AA"})
		s.requireStatus(resp, http.StatusCreated)
	})

	s.step("read color back normalized", func() {
		resp := s.get(s.colorsPath())
		s.requireStatus(resp, http.StatusOK)

		var saved struct {
			Colors []struct {
				Hex       string    `json:"hex"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"colors"`
		}
		s.jsonDecode(resp, &saved)
		s.Require().Len(saved.Colors, 1)
		s.Require().Equal("#ff00aa", saved.Colors[0].Hex)
	})

	s.step("delete color", func() {
		resp := s.del(s.colorsPath() + "/ff00aa")
		s.requireStatus(resp, http.StatusNoContent)
	})

	s.step("collection is empty again", func() {
		resp := s.get(s.colorsPath())
		s.requireStatus(resp, http.StatusOK)

		var saved struct {
			Colors []struct {
				Hex string `json:"hex"`
			} `json:"colors"`
		}
		s.jsonDecode(resp, &saved)
		s.Require().Empty(saved.Colors)
	})

	s.step("delete collection", func() {
		resp := s.del(s.collectionPath())
		s.requireStatus(resp, http.StatusNoContent)
	})

	s.step("collection is gone", func() {
		resp := s.get(s.collectionPath())
		s.requireStatus(resp, http.StatusNotFound)
	})
}

func (s *E2ESuite) TestSystemEndpoints() {
	s.Require().Empty(
		s.token,
		"expect empty token")

	s.step("livez", func() {
		resp := s.get("/livez")
		s.requireStatus(resp, http.StatusOK)
	})

	s.step("readyz", func() {
		resp := s.get("/readyz")
		s.requireStatus(resp, http.StatusOK)
	})

	// Check for a broken -X version injection
	s.step("version is not empty", func() {
		resp := s.get("/api/v1/version")
		s.requireStatus(resp, http.StatusOK)

		var got struct {
			Version string `json:"version"`
		}
		s.jsonDecode(resp, &got)
		s.Require().NotEmpty(got.Version)
	})

	s.step("the page is served at the root", func() {
		resp := s.get("/")
		s.requireStatus(resp, http.StatusOK)
		s.Require().NotEmpty(resp.body)
	})
}

func (s *E2ESuite) TestHarmonyEndpoints() {
	s.Require().Empty(s.token, "expect empty token")

	// Exact colors are not pinned, the math is in unit tests.
	harmonies := []struct {
		name   string
		colors int
	}{
		{"complement", 1},
		{"split-complement", 2},
		{"triad", 2},
		{"analogous", 2},
		{"square", 3},
		{"ramp", 7},
		{"tones", 7},
	}

	// not using step, independent queries
	for _, h := range harmonies {
		s.Run(h.name, func() {
			resp := s.get("/api/v1/colors/ff0000/" + h.name)
			s.requireStatus(resp, http.StatusOK)

			var got harmonyJSON
			s.jsonDecode(resp, &got)
			s.Require().Equal("#ff0000", got.Hex)
			s.Require().Equal(h.name, got.Harmony)
			s.Require().Len(got.Colors, h.colors)

		})
	}

	s.step("wrong hex is rejected", func() {
		resp := s.get("/api/v1/colors/nothex/complement")
		s.requireStatus(resp, http.StatusBadRequest)
	})

	s.step("unknown harmony is rejected", func() {
		resp := s.get("/api/v1/colors/ff0000/plaid")
		s.requireStatus(resp, http.StatusNotFound)
	})
}

func (s *E2ESuite) TestCSSColorsEndpoint() {
	s.Require().Empty(s.token, "expect empty token")

	var byName []commonColorJSON

	s.step("palette is 100 CSS colors", func() {
		byName = s.commonColors("")
		s.Require().Len(byName, 100)
		for _, c := range byName {
			s.Require().NotEmpty(c.Name)
		}
	})

	s.step("default sort is by name", func() {
		s.Require().True(
			slices.IsSortedFunc(byName, func(a, b commonColorJSON) int {
				return strings.Compare(a.Name, b.Name)
			}),
			"default listing is not sorted by name")
		s.Require().Equal(byName, s.commonColors("sort=name"))
	})

	s.step("sort by hex", func() {
		byHex := s.commonColors("sort=hex")
		s.Require().True(
			slices.IsSortedFunc(byHex, func(a, b commonColorJSON) int {
				return strings.Compare(a.Hex, b.Hex)
			}),
			"sort=hex is not sorted by hex")
	})

	// not checking exact order
	s.step("sort by color", func() {
		byFeel := s.commonColors("sort=color")
		s.Require().ElementsMatch(byName, byFeel)
		s.Require().NotEqual(byName, byFeel)
	})

	s.step("order desc reverses", func() {
		desc := s.commonColors("sort=hex&order=desc")
		slices.Reverse(desc)
		s.Require().Equal(s.commonColors("sort=hex&order=asc"), desc)
	})

	s.step("wrong sort and order are rejected", func() {
		for _, query := range []string{"sort=created_at", "sort=names", "order=sideways"} {
			resp := s.get("/api/v1/colors?" + query)
			s.requireStatus(resp, http.StatusBadRequest)
		}
	})
}

// commonColors reads GET /api/v1/colors.
// query is a raw query string, empty means default.
func (s *E2ESuite) commonColors(query string) []commonColorJSON {
	path := "/api/v1/colors"
	if query != "" {
		path += "?" + query
	}

	resp := s.get(path)
	s.requireStatus(resp, http.StatusOK)

	var list struct {
		Colors []commonColorJSON `json:"colors"`
	}
	s.jsonDecode(resp, &list)
	return list.Colors
}
