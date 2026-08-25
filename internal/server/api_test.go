package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/salandered/wavelen/internal/apispec"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/requestid"
	"github.com/salandered/wavelen/internal/server"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/suite"
)

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APISuite))
}

type APISuite struct {
	suite.Suite
	server  *httptest.Server
	client  *http.Client
	router  routers.Router
	storage *mockStorage
}

func (s *APISuite) SetupSuite() {
	slog.SetDefault(slog.New(slog.DiscardHandler)) // the handlers log every rejection

	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(apispec.Spec)
	s.Require().NoError(err)
	s.Require().NoError(spec.Validate(loader.Context)) // check if api.yaml is ok

	s.router, err = gorillamux.NewRouter(spec)
	s.Require().NoError(err)
}

func (s *APISuite) SetupTest() {
	s.storage = newMockStorage()
	s.server = httptest.NewServer(server.NewHandler(s.storage))
	s.client = s.server.Client()
}

func (s *APISuite) TearDownTest() {
	s.server.Close()
}

// Routing and middleware

func (s *APISuite) TestRootReturnsServiceNameAndVersion() {
	resp := s.get("/")
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	// the version itself is build-time injected
	s.Require().Contains(s.body(resp), "wavelen version")
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

func (s *APISuite) TestReadyzReturnsServiceUnavailableAndNamesTheDependency() {
	s.storage.pingErr = errors.New("connection refused")

	resp := s.get("/readyz")
	s.Require().Equal(http.StatusServiceUnavailable, resp.StatusCode)
	s.Require().JSONEq(`{"status":"unavailable","dependency":"postgres"}`, s.body(resp))
}

func (s *APISuite) TestUnknownPathReturnsNotFound() {
	resp := s.get("/no-such-path")
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
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

func (s *APISuite) TestCreateUserReturnsCreatedWithLocation() {
	s.storage.assignID = 7

	resp := s.post("/api/v1/users", handlers.CreateUserReq{
		Email: "ada@example.com",
		Name:  "Ada Lovelace",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.Require().Equal("/api/v1/users/7", resp.Header.Get("Location"))

	var out handlers.CreateUserResp
	s.decode(resp, &out)
	s.Require().Equal(int64(7), out.User.ID)
	s.Require().Equal("ada@example.com", out.User.Email)
	s.Require().Equal("Ada Lovelace", out.User.Name)
	s.Require().Equal(stubTime.UTC(), out.User.CreatedAt)
}

func (s *APISuite) TestCreateUserPassesNormalizedFieldsToStorage() {
	resp := s.post("/api/v1/users", handlers.CreateUserReq{
		Email: "  Ada@Example.COM ",
		Name:  "  Ada  ",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal("ada@example.com", s.storage.gotUser.Email)
	s.Require().Equal("Ada", s.storage.gotUser.Name)
}

func (s *APISuite) TestCreateUserMapsDuplicateEmailToConflict() {
	s.storage.createErr = storage.ErrDuplicateEmail

	resp := s.post("/api/v1/users", handlers.CreateUserReq{Email: "ada@example.com", Name: "Ada"})
	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("email already registered", s.errorMessage(resp))
}

func (s *APISuite) TestCreateUserRejectsBadInput() {
	tests := map[string]string{
		"invalid email": `{"email":"not-an-email","name":"Ada"}`,
		"empty name":    `{"email":"ada@example.com","name":"   "}`,
		"unknown field": `{"email":"ada@example.com","name":"Ada","admin":true}`,
		"empty body":    ``,
		"not an object": `["ada@example.com"]`,
		"two objects":   `{"email":"a@b.com","name":"A"}{"email":"c@d.com","name":"C"}`,
	}
	for name, body := range tests {
		s.Run(name, func() {
			resp := s.postRaw("/api/v1/users", body)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Nil(s.storage.gotUser, "storage should not be reached")
		})
	}
}

// Adding a color

func (s *APISuite) TestAddColorReturnsCreatedWhenStorageAddedIt() {
	s.storage.added = true

	resp := s.post("/api/v1/users/1/colors", handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
}

func (s *APISuite) TestAddColorReturnsOKWhenAlreadySaved() {
	s.storage.added = false

	resp := s.post("/api/v1/users/1/colors", handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	// same body either way, only the status differs
	var out handlers.AddColorResp
	s.decode(resp, &out)
	s.Require().Equal("#ff0000", out.Hex)
}

func (s *APISuite) TestAddColorPassesNormalizedHexAndPathIDToStorage() {
	resp := s.post("/api/v1/users/42/colors", handlers.AddColorReq{Hex: "  FF00AA  "})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	s.Require().Equal(color.Hex("#ff00aa"), s.storage.gotHex)
	s.Require().Equal(user.ID(42), s.storage.gotUserID)

	var out handlers.AddColorResp
	s.decode(resp, &out)
	s.Require().Equal("#ff00aa", out.Hex)
}

func (s *APISuite) TestAddColorMapsUnknownUserToNotFound() {
	s.storage.addErr = storage.ErrUserNotFound

	resp := s.post("/api/v1/users/999/colors", handlers.AddColorReq{Hex: "#ff0000"})
	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Equal("user not found", s.errorMessage(resp))
}

func (s *APISuite) TestAddColorRejectsBadHex() {
	for _, hex := range []string{"", "#fff", "red", "#ff00gg", "#ff0000ff"} {
		s.Run(hex, func() {
			resp := s.post("/api/v1/users/1/colors", handlers.AddColorReq{Hex: hex})
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			s.Require().Empty(s.storage.gotHex, "storage should not be reached")
		})
	}
}

func (s *APISuite) TestMalformedUserIDReturnsBadRequest() {
	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		s.Run(id, func() {
			s.Require().Equal(http.StatusBadRequest, s.get("/api/v1/users/"+id+"/colors").StatusCode)
			s.Require().Equal(http.StatusBadRequest,
				s.post("/api/v1/users/"+id+"/colors", handlers.AddColorReq{Hex: "#ff0000"}).StatusCode)
			s.Require().Zero(s.storage.gotUserID, "storage should not be reached")
		})
	}
}

// Listing colors

func (s *APISuite) TestListColorsRendersEmptyArrayNotNull() {
	s.storage.colors = nil

	resp := s.get("/api/v1/users/1/colors")
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().JSONEq(`{"colors":[],"metadata":{"limit":50}}`, s.body(resp))
}

func (s *APISuite) TestListColorsKeepsRepoOrderAndNormalizesTimeToUTC() {
	s.storage.colors = []color.Color{
		{Hex: "#0000ff", CreatedAt: stubTime},
		{Hex: "#00ff00", CreatedAt: stubTime.Add(-time.Hour)},
	}

	var out handlers.ListColorsResp
	s.decode(s.get("/api/v1/users/1/colors"), &out)

	s.Require().Len(out.Colors, 2)
	s.Require().Equal("#0000ff", out.Colors[0].Hex)
	s.Require().Equal("#00ff00", out.Colors[1].Hex)
	s.Require().Equal(time.UTC, out.Colors[0].CreatedAt.Location())
}

func (s *APISuite) TestListColorsAppliesDefaultsWhenNoParamsAreGiven() {
	s.get("/api/v1/users/1/colors")

	s.Require().Equal(storage.ListColorsParams{
		Sort:  storage.SortByCreatedAt,
		Order: storage.OrderDesc,
		Limit: 50,
	}, s.storage.gotParams)
}

func (s *APISuite) TestListColorsPassesEveryParsedParamDownIncludingACursorRoundTrip() {
	s.storage.colors = []color.Color{{Hex: "#00ff00", CreatedAt: stubTime}}
	s.storage.hasMore = true

	var first handlers.ListColorsResp
	s.decode(s.get("/api/v1/users/1/colors?sort=hex&order=asc&limit=1"), &first)
	s.Require().NotEmpty(first.Metadata.NextCursor)
	s.Require().Equal(1, first.Metadata.Limit)

	// when the client feeds that cursor back
	s.get("/api/v1/users/1/colors?sort=hex&order=asc&limit=1&cursor=" + first.Metadata.NextCursor)

	got := s.storage.gotParams
	s.Require().Equal(storage.SortByHex, got.Sort)
	s.Require().Equal(storage.OrderAsc, got.Order)
	s.Require().Equal(1, got.Limit)
	s.Require().NotNil(got.After)
	s.Require().Equal(color.Hex("#00ff00"), got.After.Hex)
}

func (s *APISuite) TestListColorsPassesTheColorSortDownAndRoundTripsItsCursor() {
	s.storage.colors = []color.Color{{Hex: "#00ff00", CreatedAt: stubTime}}
	s.storage.hasMore = true

	var first handlers.ListColorsResp
	s.decode(s.get("/api/v1/users/1/colors?sort=color&order=asc&limit=1"), &first)
	s.Require().Equal(storage.SortByColor, s.storage.gotParams.Sort)
	s.Require().NotEmpty(first.Metadata.NextCursor)

	s.get("/api/v1/users/1/colors?sort=color&order=asc&limit=1&cursor=" + first.Metadata.NextCursor)

	got := s.storage.gotParams
	s.Require().Equal(storage.SortByColor, got.Sort)
	s.Require().NotNil(got.After)
	s.Require().Equal(color.Hex("#00ff00"), got.After.Hex)
}

func (s *APISuite) TestListColorsOmitsNextCursorAtTheEndOfTheList() {
	s.storage.colors = []color.Color{{Hex: "#00ff00", CreatedAt: stubTime}}
	s.storage.hasMore = false

	var out handlers.ListColorsResp
	s.decode(s.get("/api/v1/users/1/colors"), &out)

	s.Require().Empty(out.Metadata.NextCursor)
}

func (s *APISuite) TestListColorsRejectsUnusableQueryParams() {
	for _, query := range []string{
		"limit=0", "limit=101", "limit=all",
		"sort=name", "order=sideways",
		"cursor=!!!", "sort=hex&cursor=" + encodedCreatedAtCursor,
	} {
		s.Run(query, func() {
			resp := s.get("/api/v1/users/1/colors?" + query)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
		})
	}
}

// created under created_at, any request sorting by hex must reject it
const encodedCreatedAtCursor = "Y3JlYXRlZF9hdHxkZXNjfDIwMjYtMDgtMjNUMTQ6MDA6MDBafCNmZjAwYWE"

// The shared palette

func (s *APISuite) TestListCommonColorsRendersTheCatalog() {
	s.storage.common = []color.Common{
		{Hex: "#000000", Name: "black"},
		{Hex: "#ff0000", Name: "red"},
	}

	var out handlers.ListCommonColorsResp
	s.decode(s.get("/api/v1/colors"), &out)

	s.Require().Len(out.Colors, 2)
	s.Require().Equal(handlers.CommonColorResp{Hex: "#000000", Name: "black"}, out.Colors[0])
}

func (s *APISuite) TestListCommonColorsAppliesDefaultsWhenNoParamsAreGiven() {
	s.get("/api/v1/colors")

	s.Require().Equal(storage.ListCommonColorsParams{
		Sort:  storage.CatalogSortByName,
		Order: storage.OrderAsc,
	}, s.storage.gotCatalogParams)
}

func (s *APISuite) TestListCommonColorsPassesTheSortDown() {
	s.get("/api/v1/colors?sort=hex&order=desc")

	s.Require().Equal(storage.ListCommonColorsParams{
		Sort:  storage.CatalogSortByHex,
		Order: storage.OrderDesc,
	}, s.storage.gotCatalogParams)
}

func (s *APISuite) TestListCommonColorsPassesTheColorSortDown() {
	s.get("/api/v1/colors?sort=color")

	s.Require().Equal(storage.ListCommonColorsParams{
		Sort:  storage.CatalogSortByColor,
		Order: storage.OrderAsc,
	}, s.storage.gotCatalogParams)
}

// created_at is a column of user_colors only, the palette has no such column
func (s *APISuite) TestListCommonColorsRejectsUnusableQueryParams() {
	for _, query := range []string{"sort=created_at", "sort=names", "order=sideways"} {
		s.Run(query, func() {
			resp := s.get("/api/v1/colors?" + query)
			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
		})
	}
}

// Failure mapping

func (s *APISuite) TestStorageFailureReturnsServerErrorWithoutLeakingIt() {
	s.storage.commonErr = errors.New("connection refused to 10.0.0.5:5432")

	resp := s.get("/api/v1/colors")
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

func (s *APISuite) send(method, path string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, s.server.URL+path, body)
	s.Require().NoError(err)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	s.T().Cleanup(func() { resp.Body.Close() })

	s.validateAgainstSpec(resp)
	return resp
}

// Checks that resp satisfies internal/apispec/api.yaml.
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
