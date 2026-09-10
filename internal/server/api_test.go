package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/salandered/wavelen"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/icon"
	"github.com/salandered/wavelen/internal/requestid"
	"github.com/salandered/wavelen/internal/server"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
	"github.com/salandered/wavelen/internal/usersvc"
	"github.com/stretchr/testify/suite"
)

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APISuite))
}

var (
	collectionsPath = "/api/v1/me/collections"
	collectionPath  = collectionsPath + "/" + stubCollectionID.String()
	savedColorsPath = collectionPath + "/colors"
)

const (
	testQuota           = 3
	testCollectionQuota = 2
	testTTL             = time.Hour
	testPassword        = "correct horse battery"
	testToken           = "X3ASTT2CDAN66BACKSCI4SU7SI"

	testAuthConcurLimit = 2
	testAuthConcurWait  = 50 * time.Millisecond
)

type APISuite struct {
	suite.Suite
	server  *httptest.Server
	client  *http.Client
	router  routers.Router
	storage *mockStorage
}

func (s *APISuite) SetupSuite() {
	slog.SetDefault(slog.New(slog.DiscardHandler)) // handlers log every rejection

	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(wavelen.APISpec)
	s.Require().NoError(err)
	s.Require().NoError(spec.Validate(loader.Context)) // check if api.yaml is ok

	s.router, err = gorillamux.NewRouter(spec)
	s.Require().NoError(err)
}

func (s *APISuite) SetupTest() {
	s.storage = newMockStorage()
	s.server = httptest.NewServer(server.NewHandler(s.storage, server.HandlerConfig{
		WebFS:               stubWebFS,
		UserColorQuota:      testQuota,
		UserCollectionQuota: testCollectionQuota,
		AuthTokenTTL:        testTTL,
		AuthConcurLimit:     testAuthConcurLimit,
		AuthConcurWait:      testAuthConcurWait,
	}))
	s.client = s.server.Client()
}

func (s *APISuite) TearDownTest() {
	s.server.Close()
}

func (s *APISuite) SetupSubTest() {
	s.storage.reset()
}

// Routing and middleware

var stubWebFS = fstest.MapFS{
	"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>wavelen</title>")},
	"app.js":     &fstest.MapFile{Data: []byte("export const stub = true")},
}

func (s *APISuite) TestRootServesTheUI() {
	resp := s.get("/")
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Contains(resp.Header.Get("Content-Type"), "text/html")
	s.Require().Contains(s.body(resp), "<title>wavelen</title>")
}

// Not using s.get which is validated against the spec. api.yaml only specifies
// "/" path (not "/app.js"), so there is nothing to validate the resp against.
func (s *APISuite) TestAssetsAreServedNextToTheAPI() {
	resp, err := s.client.Get(s.server.URL + "/app.js")
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Contains(s.body(resp), "stub")
}

func (s *APISuite) TestWritingToAssetIsNotFound() {
	// [handlers.readOnly] should catch it
	resp := s.post("/app.js", nil)
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *APISuite) TestVersionReturnsTheBuildVersion() {
	resp := s.get("/api/v1/version")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var out handlers.VersionResp
	s.decode(resp, &out)
	// the version value is build-time injected
	s.Require().NotEmpty(out.Version)
}

func (s *APISuite) TestLivezReturnsOkWithoutTouchingStorage() {
	resp := s.get("/livez")
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().JSONEq(`{"status":"ok"}`, s.body(resp))
	s.Require().Zero(s.storage.pingCalls)
}

func (s *APISuite) TestReadyzReturnsOkWhenTheDatabaseAnswers() {
	resp := s.get("/readyz")
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().JSONEq(`{"status":"ok"}`, s.body(resp))
	s.Require().Equal(1, s.storage.pingCalls)
}

func (s *APISuite) TestReadyzReturnsServiceUnavailableAndNamesDependency() {
	s.storage.pingErr = errors.New("connection refused")

	resp := s.get("/readyz")
	s.Require().Equal(http.StatusServiceUnavailable, resp.StatusCode)
	s.Require().JSONEq(`{"status":"unavailable","dependency":"postgres"}`, s.body(resp))
}

func (s *APISuite) TestUnknownPathReturnsNotFound() {
	// NewStaticHandler catches this
	for _, path := range []string{"/no-such-path", "/api/v1/no-such-path"} {
		s.Run(path, func() {
			resp := s.get(path)
			s.Require().Equal(http.StatusNotFound, resp.StatusCode)
		})
	}
}

func (s *APISuite) TestRequestIDIsGeneratedAndClientHeaderIgnored() {
	req, err := http.NewRequest(http.MethodGet, s.server.URL+"/", nil)
	s.Require().NoError(err)
	req.Header.Set(requestid.Header, "client-sent-id")

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	got := resp.Header.Get(requestid.Header)
	s.Require().NotEmpty(got)
	s.Require().NotEqual("client-sent-id", got)
}

// Users

func (s *APISuite) TestCreateUserReturnsCreated() {
	resp := s.post("/api/v1/users", handlers.CreateUserReq{
		Nickname: "olya",
		Name:     "Olya Lovelace",
		Password: testPassword,
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var out handlers.CreateUserResp
	s.decode(resp, &out)
	s.Require().Equal("olya", out.User.Nickname)
	s.Require().Equal("Olya Lovelace", out.User.Name)
	s.Require().Equal(stubTime.UTC(), out.User.CreatedAt)
}

func (s *APISuite) TestCreateUserPassesNormalizedFieldsToStorage() {
	resp := s.post("/api/v1/users", handlers.CreateUserReq{
		Nickname: "  Olya  ",
		Name:     "  Olya  ",
		Password: testPassword,
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal("olya", s.storage.gotUser.Nickname)
	s.Require().Equal("Olya", s.storage.gotUser.Name)
}

func (s *APISuite) TestCreateUserAlsoCreatesTheDefaultCollection() {
	resp := s.post("/api/v1/users", handlers.CreateUserReq{
		Nickname: "olya",
		Name:     "Olya",
		Password: testPassword,
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal(usersvc.DefCollectionName, s.storage.gotCltParams.Name)
	s.Require().Equal(collection.DefIconSlug, s.storage.gotCltParams.Icon)
	s.Require().Equal(collection.DefIconAccent, s.storage.gotCltParams.Accent)
}

func (s *APISuite) TestCreateUserDuplicateNicknameReturnsConflict() {
	s.storage.createErr = storage.ErrDuplicateNickname

	resp := s.post("/api/v1/users", handlers.CreateUserReq{
		Nickname: "olya", Name: "Olya", Password: testPassword,
	})
	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("nickname already taken", s.errorMessage(resp))
}

func (s *APISuite) TestGetMeReturnsTheAccountTheTokenBelongsTo() {
	s.storage.tokenUser = 7
	s.storage.userByID = &user.User{
		ID: 7, Nickname: "olya", Name: "Olya Lovelace", CreatedAt: stubTime,
	}

	resp := s.get("/api/v1/me")

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var out handlers.MeResp
	s.decode(resp, &out)
	s.Require().Equal("olya", out.User.Nickname)
	s.Require().Equal("Olya Lovelace", out.User.Name)
	s.Require().Equal(stubTime.UTC(), out.User.CreatedAt)
	// the id came from the token, the request carried none
	s.Require().Equal(user.ID(7), s.storage.gotUserID)
}

func (s *APISuite) TestGetMeWithoutCredentialsIsUnauthorized() {
	resp := s.sendAs(http.MethodGet, "/api/v1/me", nil, "")

	s.Require().Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Require().Zero(s.storage.gotUserID)
}

// Unreachable in production, tokens cascade with the user.
func (s *APISuite) TestGetMeUnknownUserReturnsNotFound() {
	s.storage.idErr = storage.ErrUserNotFound

	resp := s.get("/api/v1/me")

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("user not found", s.errorMessage(resp))
}

func (s *APISuite) TestCreateTokenReturnsTokenAndStoresTheHash() {
	hash, err := auth.HashPassword(testPassword)
	s.Require().NoError(err)
	s.storage.userByNick = &user.User{ID: 7, Nickname: "olya", PasswordHash: hash}

	// when
	resp := s.post("/api/v1/tokens", handlers.CreateTokenReq{
		Nickname: "  Olya  ", Password: testPassword,
	})

	// then
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var out handlers.CreateTokenResp
	s.decode(resp, &out)
	s.Require().NotEmpty(out.Token)
	// was normalized before the lookup
	s.Require().Equal("olya", s.storage.gotNickname)
	s.Require().Equal(auth.HashToken(out.Token), s.storage.gotToken.Hash)
	s.Require().Equal(user.ID(7), s.storage.gotToken.UserID)
}

func (s *APISuite) TestDeleteTokenRevokesTheOneTheRequestCarried() {
	resp := s.del("/api/v1/tokens")

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
	s.Require().Equal(auth.HashToken(testToken), s.storage.deletedTokenHash)
}

func (s *APISuite) TestDeleteTokenWithoutCredentialsRevokesNothing() {
	resp := s.sendAs(http.MethodDelete, "/api/v1/tokens", nil, "")

	s.Require().Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Require().Nil(s.storage.deletedTokenHash)
}

func (s *APISuite) TestDeleteTokenWithAnAlreadyInvalidTokenIsUnauthorized() {
	s.storage.tokenErr = storage.ErrTokenNotFound

	resp := s.del("/api/v1/tokens")

	s.Require().Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Require().Nil(s.storage.deletedTokenHash)
}

func (s *APISuite) TestCreateTokenAnswersTheSameForWrongPasswordAndUnknownNickname() {
	hash, err := auth.HashPassword(testPassword)
	s.Require().NoError(err)
	s.storage.userByNick = &user.User{ID: 7, Nickname: "olya", PasswordHash: hash}

	wrongPassword := s.post("/api/v1/tokens", handlers.CreateTokenReq{
		Nickname: "olya", Password: "not the password",
	})

	s.storage.userByNick, s.storage.nickErr = nil, storage.ErrUserNotFound
	unknownNickname := s.post("/api/v1/tokens", handlers.CreateTokenReq{
		Nickname: "nobody", Password: testPassword,
	})

	s.Require().Equal(http.StatusUnauthorized, wrongPassword.StatusCode)
	s.Require().Equal(http.StatusUnauthorized, unknownNickname.StatusCode)
	s.Require().Equal(s.errorMessage(wrongPassword), s.errorMessage(unknownNickname))
	s.Require().Nil(s.storage.gotToken)
}

func (s *APISuite) TestCreateUserRejectsBadInput() {
	tests := map[string]string{
		"invalid nickname": `{"nickname":"olya lovelace","name":"Olya","password":"correct horse battery"}`,
		"short nickname":   `{"nickname":"ol","name":"Olya","password":"correct horse battery"}`,
		"empty name":       `{"nickname":"olya","name":"   ","password":"correct horse battery"}`,
		"short password":   `{"nickname":"olya","name":"Olya","password":"short"}`,
		"missing password": `{"nickname":"olya","name":"Olya"}`,
		"unknown field":    `{"nickname":"olya","name":"Olya","admin":true}`,
		"empty body":       ``,
		"not an object":    `["olya"]`,
		"two objects":      `{"nickname":"a","name":"A"}{"nickname":"c","name":"C"}`,
	}
	for name, body := range tests {
		s.Run(name, func() {
			resp := s.postRaw("/api/v1/users", body)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Nil(s.storage.gotUser)
		})
	}
}

// Collections

func (s *APISuite) TestCreateCollectionReturnsCreated() {
	s.storage.tokenUser = 42

	resp := s.post(collectionsPath, handlers.CreateCollectionReq{Name: "Sunset palette"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var out handlers.OneCollectionResp
	s.decode(resp, &out)
	s.Require().Equal(stubCollectionID.String(), out.Collection.ID)
	s.Require().Equal("Sunset palette", out.Collection.Name)
	s.Require().Equal(stubTime.UTC(), out.Collection.CreatedAt)
	// the id came from the token, the request carried none
	s.Require().Equal(user.ID(42), s.storage.gotUserID)
}

func (s *APISuite) TestCreateCollectionIsNotDefault() {
	resp := s.post(collectionsPath, handlers.CreateCollectionReq{Name: "Sunset"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var out handlers.OneCollectionResp
	s.decode(resp, &out)
	s.Require().False(out.Collection.IsDefault)
}

func (s *APISuite) TestCreateCollectionPassesTrimmedNameToStorage() {
	resp := s.post(collectionsPath, handlers.CreateCollectionReq{Name: "  Sunset  "})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal("Sunset", s.storage.gotCltParams.Name)
}

func (s *APISuite) TestCreateCollectionDefaultIconAndAccentWhenOmitted() {
	resp := s.post(collectionsPath, handlers.CreateCollectionReq{Name: "Sunset"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var out handlers.OneCollectionResp
	s.decode(resp, &out)
	s.Require().Equal(string(collection.DefIconSlug), out.Collection.Icon)
	s.Require().Equal(string(collection.DefIconAccent), out.Collection.Accent)
}

func (s *APISuite) TestCreateCollectionPassesVerbatimIconAndNormalizedAccentToStorage() {
	resp := s.post(collectionsPath, handlers.CreateCollectionReq{
		Name:   "Sunset",
		Icon:   "star",
		Accent: "FF00AA",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal(icon.Slug("star"), s.storage.gotCltParams.Icon)
	s.Require().Equal(color.Hex("#ff00aa"), s.storage.gotCltParams.Accent)
}

func (s *APISuite) TestCreateCollectionRejectsBadInput() {
	tests := map[string]string{
		"empty name":     `{"name":""}`,
		"blank name":     `{"name":"   "}`,
		"overlong name":  `{"name":"` + strings.Repeat("a", collection.MaxNameLen+1) + `"}`,
		"missing name":   `{}`,
		"unknown field":  `{"name":"Sunset","is_default":true}`,
		"unknown icon":   `{"name":"Sunset","icon":"folder-open"}`,
		"uppercase icon": `{"name":"Sunset","icon":"STAR"}`,
		"padded icon":    `{"name":"Sunset","icon":" star "}`,
		"icon markup":    `{"name":"Sunset","icon":"<svg/>"}`,
		"bad accent":     `{"name":"Sunset","accent":"zzzzzz"}`,
		"blank accent":   `{"name":"Sunset","accent":"   "}`,
		"empty body":     ``,
		"not an object":  `["Sunset"]`,
	}
	for name, body := range tests {
		s.Run(name, func() {
			resp := s.postRaw(collectionsPath, body)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Empty(s.storage.gotCltParams)
		})
	}
}

func (s *APISuite) TestCreateCollectionAtTheQuotaReturnsConflict() {
	s.storage.collectionCount = testCollectionQuota

	resp := s.post(collectionsPath, handlers.CreateCollectionReq{Name: "Sunset"})

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("collection quota full", s.errorMessage(resp))
}

func (s *APISuite) TestCreateCollectionForUnknownUserReturnsNotFound() {
	s.storage.lockUserErr = storage.ErrUserNotFound

	resp := s.post(collectionsPath, handlers.CreateCollectionReq{Name: "Sunset"})

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("user not found", s.errorMessage(resp))
}

func (s *APISuite) TestListCollectionsRendersEmptyArrayNotNull() {
	s.storage.collections = nil

	resp := s.get(collectionsPath)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().JSONEq(`{"collections":[]}`, s.body(resp))
}

func (s *APISuite) TestListCollectionsKeepsOrderAndNormalizesTimeToUTC() {
	s.storage.collections = []collection.Collection{
		{
			ID:         stubCollectionID,
			Name:       "Main",
			IconSlug:   collection.DefIconSlug,
			IconAccent: collection.DefIconAccent,
			IsDefault:  true,
			CreatedAt:  stubTime,
		},
		{
			ID:         otherCollectionID,
			Name:       "Sunset",
			IconSlug:   "star",
			IconAccent: "#ff00aa",
			CreatedAt:  stubTime.Add(time.Hour),
		},
	}

	var out handlers.ListCollectionsResp
	s.decode(s.get(collectionsPath), &out)

	s.Require().Len(out.Collections, 2)
	s.Require().Equal("Main", out.Collections[0].Name)
	s.Require().True(out.Collections[0].IsDefault)
	s.Require().Equal(otherCollectionID.String(), out.Collections[1].ID)
	s.Require().False(out.Collections[1].IsDefault)
	s.Require().Equal(time.UTC, out.Collections[0].CreatedAt.Location())
}

func (s *APISuite) TestGetCollection() {
	s.storage.tokenUser = 42

	resp := s.get(collectionPath)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var out handlers.OneCollectionResp
	s.decode(resp, &out)
	s.Require().Equal(stubCollectionID.String(), out.Collection.ID)
	s.Require().Equal(user.ID(42), s.storage.gotUserID)
	s.Require().Equal(stubCollectionID, s.storage.gotCollectionID)
}

func (s *APISuite) TestGetCollectionTheCallerDoesNotOwnReturnsNotFound() {
	s.storage.resolveErr = storage.ErrNotFound

	resp := s.get(collectionPath)

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("not found", s.errorMessage(resp))
}

func (s *APISuite) TestCollectionRoutesRejectAMalformedID() {
	for _, path := range []string{
		"/api/v1/me/collections/main",
		"/api/v1/me/collections/42",
		"/api/v1/me/collections/not-a-uuid/colors",
	} {
		s.Run(path, func() {
			resp := s.get(path)

			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Contains(s.errorMessage(resp), "invalid collection id")
		})
	}
}

func (s *APISuite) TestDeleteCollectionReturnsNoContent() {
	s.storage.tokenUser = 42

	resp := s.del(collectionPath)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
	s.Require().Empty(s.body(resp))
	s.Require().Equal(user.ID(42), s.storage.gotUserID)
	s.Require().Equal(stubCollectionID, s.storage.gotCollectionID)
}

func (s *APISuite) TestDeleteDefaultCollectionReturnsConflict() {
	s.storage.cltIsDefault = true

	resp := s.del(collectionPath)

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("default collection cannot be deleted", s.errorMessage(resp))
}

func (s *APISuite) TestDeleteCollectionTheCallerDoesNotOwnReturnsNotFound() {
	s.storage.resolveErr = storage.ErrNotFound

	resp := s.del(collectionPath)

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("not found", s.errorMessage(resp))
}

// Adding a color

func (s *APISuite) TestAddColorReturnsCreated() {
	s.storage.colorAdded = true

	resp := s.post(savedColorsPath, handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
}

func (s *APISuite) TestAddColorReturnsOKWhenAlreadySaved() {
	s.storage.colorAdded = false

	resp := s.post(savedColorsPath, handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	// same body either way, only the status differs
	var out handlers.AddColorResp
	s.decode(resp, &out)
	s.Require().Equal("#ff0000", out.Hex)
}

func (s *APISuite) TestAddColorPassesNormalizedHexAndTokenUserToStorage() {
	s.storage.tokenUser = 42

	resp := s.post(savedColorsPath, handlers.AddColorReq{Hex: "  FF00AA  "})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal(color.Hex("#ff00aa"), s.storage.gotHex)
	s.Require().Equal(user.ID(42), s.storage.gotUserID)

	var out handlers.AddColorResp
	s.decode(resp, &out)
	s.Require().Equal("#ff00aa", out.Hex)
}

func (s *APISuite) TestAddColorToCollectionCallerDoesNotOwnReturnsNotFound() {
	s.storage.resolveErr = storage.ErrNotFound

	s.storage.tokenUser = 999

	resp := s.post(savedColorsPath, handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("not found", s.errorMessage(resp))
}

func (s *APISuite) TestAddColorFullQuotaShouldReturnConflict() {
	s.storage.colorCount = testQuota

	resp := s.post(savedColorsPath, handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("color quota full", s.errorMessage(resp))
}

func (s *APISuite) TestAddColorRejectsBadHex() {
	for _, hex := range []string{"", "#fff", "red", "#ff00gg", "#ff0000ff"} {
		s.Run(hex, func() {
			resp := s.post(savedColorsPath, handlers.AddColorReq{Hex: hex})
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Empty(s.storage.gotHex)
		})
	}
}

// Deleting a color

func (s *APISuite) TestDeleteColorReturnsNoContent() {
	s.storage.tokenUser = 42

	resp := s.del(savedColorsPath + "/FF00AA")
	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
	s.Require().Empty(s.body(resp))

	s.Require().Equal(user.ID(42), s.storage.gotUserID)
	s.Require().Equal(color.Hex("#ff00aa"), s.storage.gotHex)
}

func (s *APISuite) TestDeleteColorRejectsAnEscapedHash() {
	resp := s.del(savedColorsPath + "/%23ff00aa")
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.Require().Empty(s.storage.gotHex)
}

// A '#' sent unescaped is a fragment: the server sees an empty segment and no route.
func (s *APISuite) TestDeleteColorWithAnEmptyHexSegmentIsNotFound() {
	s.Require().Equal(http.StatusNotFound, s.del(savedColorsPath+"/").StatusCode)
	s.Require().Empty(s.storage.gotHex)
}

func (s *APISuite) TestDeleteColorUserDoesNotHaveReturnsNotFound() {
	s.storage.deleteColorErr = storage.ErrNotFound

	resp := s.del(savedColorsPath + "/ff0000")
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("not found", s.errorMessage(resp))
}

func (s *APISuite) TestDeleteColorRejectsBadHex() {
	for _, hex := range []string{"fff", "red", "ff00gg", "ff0000ff", "%23fff"} {
		s.Run(hex, func() {
			resp := s.del(savedColorsPath + "/" + hex)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Empty(s.storage.gotHex)
		})
	}
}

// Listing colors

func (s *APISuite) TestListColorsRendersEmptyArrayNotNull() {
	s.storage.colors = nil

	resp := s.get(savedColorsPath)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().JSONEq(`{"colors":[],"metadata":{"limit":50}}`, s.body(resp))
}

func (s *APISuite) TestListColorsKeepsRepoOrderAndNormalizesTimeToUTC() {
	s.storage.colors = []color.Color{
		{Hex: "#0000ff", CreatedAt: stubTime},
		{Hex: "#00ff00", CreatedAt: stubTime.Add(-time.Hour)},
	}

	var out handlers.ListColorsResp
	s.decode(s.get(savedColorsPath), &out)

	s.Require().Len(out.Colors, 2)
	s.Require().Equal("#0000ff", out.Colors[0].Hex)
	s.Require().Equal("#00ff00", out.Colors[1].Hex)
	s.Require().Equal(time.UTC, out.Colors[0].CreatedAt.Location())
}

func (s *APISuite) TestListColorsAppliesDefaultsWhenNoParamsAreGiven() {
	s.get(savedColorsPath)

	s.Require().Equal(storage.ListColorsParams{
		Sort:  storage.SortByCreatedAt,
		Order: storage.OrderDesc,
		Limit: 50,
	}, s.storage.gotParams)
}

func (s *APISuite) TestListColorsPassesEveryParsedParamDownIncludingCursorRoundTrip() {
	s.storage.colors = []color.Color{{Hex: "#00ff00", CreatedAt: stubTime}}
	s.storage.hasMore = true

	var first handlers.ListColorsResp
	s.decode(s.get(savedColorsPath+"?sort=hex&order=asc&limit=1"), &first)
	s.Require().NotEmpty(first.Metadata.NextCursor)
	s.Require().Equal(1, first.Metadata.Limit)

	// when the client feeds that cursor back
	s.get(savedColorsPath + "?sort=hex&order=asc&limit=1&cursor=" + first.Metadata.NextCursor)

	got := s.storage.gotParams
	s.Require().Equal(storage.SortByHex, got.Sort)
	s.Require().Equal(storage.OrderAsc, got.Order)
	s.Require().Equal(1, got.Limit)
	s.Require().NotNil(got.After)
	s.Require().Equal(color.Hex("#00ff00"), got.After.Hex)
}

func (s *APISuite) TestListColorsOmitsNextCursorAtTheEndOfTheList() {
	s.storage.colors = []color.Color{{Hex: "#00ff00", CreatedAt: stubTime}}
	s.storage.hasMore = false

	var out handlers.ListColorsResp
	s.decode(s.get(savedColorsPath), &out)

	s.Require().Empty(out.Metadata.NextCursor)
}

func (s *APISuite) TestListColorsRejectsUnusableQueryParams() {
	for _, query := range []string{
		"limit=0", "limit=101", "limit=all",
		"sort=name", "order=sideways",
		"cursor=!!!", "sort=hex&cursor=" + encodedCreatedAtCursor,
	} {
		s.Run(query, func() {
			resp := s.get(savedColorsPath + "?" + query)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
		})
	}
}

// created under created_at, any request sorting by hex must reject it
const encodedCreatedAtCursor = "Y3JlYXRlZF9hdHxkZXNjfDIwMjYtMDgtMjNUMTQ6MDA6MDBafCNmZjAwYWE"

// The shared palette

func (s *APISuite) TestListCommonColorsRendersTheWholePalette() {
	var out handlers.ListCommonColorsResp
	s.decode(s.get("/api/v1/colors"), &out)

	s.Require().Len(out.Colors, 100)
	for _, c := range out.Colors {
		s.Require().Len(c.Hex, color.HexLen)
		s.Require().NotEmpty(c.Name)
	}
}

func (s *APISuite) TestListCommonColorsAppliesDefaultsWhenNoParamsAreGiven() {
	var out handlers.ListCommonColorsResp
	s.decode(s.get("/api/v1/colors"), &out)

	names := make([]string, 0, len(out.Colors))
	for _, c := range out.Colors {
		names = append(names, c.Name)
	}
	s.Require().True(slices.IsSorted(names))
}

func (s *APISuite) TestListCommonColorsPassesTheSortAndOrderDown() {
	var out handlers.ListCommonColorsResp
	s.decode(s.get("/api/v1/colors?sort=hex&order=desc"), &out)

	hexes := make([]string, 0, len(out.Colors))
	for _, c := range out.Colors {
		hexes = append(hexes, c.Hex)
	}
	s.Require().True(slices.IsSortedFunc(hexes, func(a, b string) int { return strings.Compare(b, a) }))
}

func (s *APISuite) TestListCommonColorsPassesTheColorSortDown() {
	var out handlers.ListCommonColorsResp
	s.decode(s.get("/api/v1/colors?sort=color"), &out)

	s.Require().Equal("black", out.Colors[0].Name) // the perceptual order opens on the neutrals
}

func (s *APISuite) TestListCommonColorsRejectsInvalidQueryParams() {
	for _, query := range []string{"sort=created_at", "sort=names", "order=sideways"} {
		s.Run(query, func() {
			resp := s.get("/api/v1/colors?" + query)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
		})
	}
}

// Harmony

func (s *APISuite) TestHarmonyEchoesInputAndNamesTheHarmony() {
	var out handlers.HarmonyResp
	s.decode(s.get("/api/v1/colors/FF0000/complement"), &out)

	want := color.Complement.Colors("#ff0000")
	s.Require().Equal("#ff0000", out.Hex)
	s.Require().Equal("complement", out.Harmony)
	s.Require().Equal([]string{string(want[0])}, out.Colors)
}

// Every harmony answers the same shape, so a client renders the list without knowing the name.
func (s *APISuite) TestEveryHarmonyAnswersTheSameShape() {
	for _, name := range color.HarmonyNames() {
		s.Run(string(name), func() {
			var out handlers.HarmonyResp
			s.decode(s.get("/api/v1/colors/ff0000/"+string(name)), &out)

			want := name.Colors("#ff0000")
			s.Require().Equal("#ff0000", out.Hex)
			s.Require().Equal(string(name), out.Harmony)
			s.Require().Len(out.Colors, len(want))
			for i, hex := range want {
				s.Require().Equal(string(hex), out.Colors[i])
			}
		})
	}
}

func (s *APISuite) TestTriadAnswersOtherTwoColorsInHueOrder() {
	var out handlers.HarmonyResp
	s.decode(s.get("/api/v1/colors/ff0000/triad"), &out)

	want := color.Triad.Colors("#ff0000")
	s.Require().Len(out.Colors, 2)
	s.Require().Equal([]string{string(want[0]), string(want[1])}, out.Colors)
}

func (s *APISuite) TestRampAnswersSevenSteps() {
	var out handlers.HarmonyResp
	s.decode(s.get("/api/v1/colors/ff6b35/ramp"), &out)

	s.Require().Len(out.Colors, 7)
}

func (s *APISuite) TestHarmonyIsCacheableForever() {
	resp := s.get("/api/v1/colors/ff0000/complement")

	s.Require().Equal("public, max-age=31536000, immutable", resp.Header.Get("Cache-Control"))
}

// A gray has no hue to turn, so it answers with itself rather than with an invented color.
func (s *APISuite) TestHarmonyOfAGrayAnswersWithThatGray() {
	for _, name := range []string{"complement", "triad", "analogous", "square"} {
		s.Run(name, func() {
			var out handlers.HarmonyResp
			s.decode(s.get("/api/v1/colors/808080/"+name), &out)

			s.Require().NotEmpty(out.Colors)
			for _, hex := range out.Colors {
				s.Require().Equal("#808080", hex)
			}
		})
	}
}

// An unknown harmony is a JSON 404 rather than the static handler's plain text one, and the body
// says what the caller could have asked for.
func (s *APISuite) TestUnknownHarmonyIsNotFoundAndListsThem() {
	resp := s.get("/api/v1/colors/ff0000/tetrad")

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	msg := s.errorMessage(resp)
	s.Require().Contains(msg, `unknown harmony "tetrad"`)
	for _, name := range color.HarmonyNames() {
		s.Require().Contains(msg, string(name))
	}
}

// Same path rule as DELETE. The hex is read before the harmony, so a bad one answers 400 even
// where the harmony is unknown too.
func (s *APISuite) TestHarmonyRejectsAMalformedHexInThePath() {
	for _, path := range []string{
		"/api/v1/colors/%23ff0000/complement",
		"/api/v1/colors/%23ff0000/triad",
		"/api/v1/colors/fff/complement",
		"/api/v1/colors/ff00gg/triad",
		"/api/v1/colors/ff00gg/tetrad",
	} {
		s.Run(path, func() {
			resp := s.get(path)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Contains(s.errorMessage(resp), "invalid hex color")
		})
	}
}

// Auth

func (s *APISuite) TestAuthenticatedRoutesRejectInvalidCreds() {
	for name, header := range map[string]string{
		"no header":      "",
		"wrong scheme":   "Basic " + testToken,
		"scheme only":    "Bearer",
		"empty token":    "Bearer ",
		"bare token":     testToken,
		"lowercase kind": "bearer " + testToken,
	} {
		s.Run(name, func() {
			resp := s.sendAs(http.MethodGet, savedColorsPath, nil, header)

			s.Require().Equal(http.StatusUnauthorized, resp.StatusCode)
			s.Require().Equal("invalid credentials", s.errorMessage(resp))
			// nothing reached the token store
			s.Require().Nil(s.storage.gotTokenHash)
		})
	}
}

func (s *APISuite) TestUnknownOrExpiredTokenIsUnauthorized() {
	s.storage.tokenErr = storage.ErrTokenNotFound

	resp := s.get(savedColorsPath)

	s.Require().Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Require().Equal("invalid credentials", s.errorMessage(resp))
	// storage is asked for the hash of the token
	s.Require().Equal(auth.HashToken(testToken), s.storage.gotTokenHash)
}

func (s *APISuite) TestPublicRoutesDoNotUseTokenStore() {
	for _, path := range []string{
		"/livez",
		"/api/v1/colors",
		"/api/v1/colors/ff0000/complement",
	} {
		s.Run(path, func() {
			resp := s.sendAs(http.MethodGet, path, nil, "")

			s.Require().Equal(http.StatusOK, resp.StatusCode)
			s.Require().Nil(s.storage.gotTokenHash)
		})
	}
}

func (s *APISuite) TestAuthenticatedResponseVariesByAuthorization() {
	resp := s.get(savedColorsPath)

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Contains(resp.Header.Values("Vary"), "Authorization")
}

// Failure mapping

func (s *APISuite) TestStorageOnFailureReturnsNoInternalInfo() {
	s.storage.colorsErr = errors.New("connection refused to 10.0.0.5:5432")

	resp := s.get(savedColorsPath)
	s.Require().Equal(http.StatusInternalServerError, resp.StatusCode)

	body := s.body(resp)
	s.Require().JSONEq(`{"error":"internal server error"}`, body)
	s.Require().NotContains(body, "10.0.0.5")
}

// Utils

func (s *APISuite) get(path string) *http.Response {
	return s.send(http.MethodGet, path, nil)
}

func (s *APISuite) post(path string, body any) *http.Response {
	raw, err := json.Marshal(body)
	s.Require().NoError(err)
	return s.send(http.MethodPost, path, bytes.NewReader(raw))
}

func (s *APISuite) postRaw(path, body string) *http.Response {
	return s.send(http.MethodPost, path, strings.NewReader(body))
}

func (s *APISuite) del(path string) *http.Response {
	return s.send(http.MethodDelete, path, nil)
}

func (s *APISuite) send(method, path string, body io.Reader) *http.Response {
	return s.sendAs(method, path, body, "Bearer "+testToken)
}

// auth is sent verbatim or omitted if empty.
func (s *APISuite) sendAs(method, path string, body io.Reader, auth string) *http.Response {
	req, err := http.NewRequest(method, s.server.URL+path, body)
	s.Require().NoError(err)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	s.T().Cleanup(func() { resp.Body.Close() })

	s.validateAgainstSpec(resp)
	return resp
}

// Checks that resp satisfies api/api.yaml.
// See https://github.com/getkin/kin-openapi#validating-http-requestsresponses
//
// Responses only. The suite sends malformed bodies on purpose to exercise the 400s,
// so validating requests would fail exactly those tests.
func (s *APISuite) validateAgainstSpec(resp *http.Response) {
	route, pathParams, err := s.router.FindRoute(resp.Request)
	if err != nil {
		// The spec describes no such route. That is agreement only when the server
		// also said 404, otherwise the code serves something api.yaml does not cover.
		s.Require().Equalf(http.StatusNotFound, resp.StatusCode,
			"%s %s: answered %d, but api.yaml has no such route",
			resp.Request.Method, resp.Request.URL.Path, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body)) // put the body back for the test itself

	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    resp.Request,
			PathParams: pathParams,
			Route:      route,
		},
		Status:  resp.StatusCode,
		Header:  resp.Header,
		Options: &openapi3filter.Options{IncludeResponseStatus: true}, // an undeclared status fails
	}
	in.SetBodyBytes(body)

	err = openapi3filter.ValidateResponse(s.T().Context(), in)
	s.Require().NoErrorf(err, "%s %s: response does not satisfy api.yaml",
		resp.Request.Method, resp.Request.URL.Path)
}

func (s *APISuite) body(resp *http.Response) string {
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return string(raw)
}

func (s *APISuite) decode(resp *http.Response, dst any) {
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(dst))
}

func (s *APISuite) errorMessage(resp *http.Response) string {
	var out struct {
		Error string `json:"error"`
	}
	s.decode(resp, &out)
	return out.Error
}
