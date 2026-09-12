//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	hostEnv     = "E2E_HOST"
	defaultHost = "http://localhost:8080"

	requestTimeout = 10 * time.Second
	testPassword   = "correct horse battery"
)

// Suite

// entrypoint
func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(E2ESuite))
}

// E2ESuite talks to a deployed server via client.
type E2ESuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
	// holds user data which is reused across tests
	nickname     string
	token        string
	collectionID string
}

func (s *E2ESuite) SetupSuite() {
	s.baseURL = baseURL()
	s.client = &http.Client{Timeout: requestTimeout}

	resp, err := s.client.Get(s.baseURL + "/readyz")
	if err != nil {
		s.T().Skipf(
			"no server at %s (%s=%q): %v",
			s.baseURL, hostEnv, os.Getenv(hostEnv), err)
	}
	defer resp.Body.Close() // after err != nil
	s.Require().Equal(
		http.StatusOK,
		resp.StatusCode,
		"server at %s is not ready", s.baseURL)
}

// SetupTest clears what the previous test left behind.
func (s *E2ESuite) SetupTest() {
	s.nickname = ""
	s.token = ""
	s.collectionID = ""
}

// TearDownTest deletes the account the test created
// (which auto deleted its data like collecton, colors, tokens via cascading).
//
// Best effort approach.
func (s *E2ESuite) TearDownTest() {
	if s.token == "" {
		return
	}

	// not using [E2ESuite.send] so there is no Require
	req, err := http.NewRequest(http.MethodDelete, s.baseURL+"/api/v1/me", nil)
	if err != nil {
		s.T().Logf("teardown: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		s.T().Logf("teardown: %v", err)
		return
	}
	defer resp.Body.Close()
	s.T().Logf("teardown: DELETE /api/v1/me -> %d", resp.StatusCode)
}

// signUpAndLogIn creates a new account and adds its token to the suite.
// Note: these endpoints are covered by a dedicated test.
func (s *E2ESuite) signUpAndLogIn() {
	s.nickname = uniqueNickname()

	resp := s.post("/api/v1/users", map[string]string{
		"nickname": s.nickname,
		"password": testPassword,
	})
	s.requireStatus(resp, http.StatusCreated)

	resp = s.post("/api/v1/tokens", map[string]string{
		"nickname": s.nickname,
		"password": testPassword,
	})
	s.requireStatus(resp, http.StatusCreated)

	var login struct {
		Token string `json:"token"`
	}
	s.jsonDecode(resp, &login)
	s.Require().NotEmpty(login.Token)

	// add to suite
	s.token = login.Token
}

// step runs a test stage as a subtest.
// A failed step stops the whole test.
// Purpose: every stage gets its own line in the output and a failed stage will be named.
func (s *E2ESuite) step(name string, fn func()) {
	/*
		Note 1
		doc: "Provides compatibility with go test pkg -run TestSuite/TestName/SubTestName."

		So we can do:
			go test ./e2e -run 'TestE2ESuite/TestAccountLifecycleFromSignupToDeletion/add_a_color'

		But it is probably useless since our steps depend on each other.

		Note 2
		doc: "The passed-in func will be executed as a subtest with a fresh instance of t."

		s.Run:

		oldT := suite.T()
		return oldT.Run(name, func(t *testing.T) {
			suite.SetT(t)
			defer suite.SetT(oldT) // <-- restored
			...

		=> the parent's *testing.T is restored, s.T().FailNow() is correct.
	*/
	if !s.Run(name, fn) {
		s.T().FailNow()
	}
}

func baseURL() string {
	host := os.Getenv(hostEnv)
	if host == "" {
		host = defaultHost
	}
	return host
}

func (s *E2ESuite) collectionPath() string {
	s.Require().NotEmpty(
		s.collectionID,
		"no collection id, expected an earlier step to set it")
	return "/api/v1/me/collections/" + s.collectionID
}

func (s *E2ESuite) colorsPath() string {
	return s.collectionPath() + "/colors"
}

// Utils

type response struct {
	method string
	path   string
	status int
	body   []byte
}

// Should satisfy [user.NormalizeNickname].
func uniqueNickname() string {
	return fmt.Sprintf("e2e%dx", time.Now().UnixNano())
}

func (s *E2ESuite) get(path string) response {
	return s.send(http.MethodGet, path, nil)
}

func (s *E2ESuite) post(path string, body any) response {
	raw, err := json.Marshal(body)
	s.Require().NoError(err)
	return s.send(http.MethodPost, path, bytes.NewReader(raw))
}

func (s *E2ESuite) del(path string) response {
	return s.send(http.MethodDelete, path, nil)
}

func (s *E2ESuite) send(method, path string, body io.Reader) response {
	req, err := http.NewRequestWithContext(s.T().Context(), method, s.baseURL+path, body)
	s.Require().NoError(err)
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return response{
		method: method,
		path:   path,
		status: resp.StatusCode,
		body:   raw,
	}
}

func (s *E2ESuite) requireStatus(resp response, want int) {
	s.Require().Equalf(
		want,
		resp.status,
		"%s %s: %s", resp.method, resp.path, resp.body,
	)
}

func (s *E2ESuite) jsonDecode(resp response, dst any) {
	s.Require().NoErrorf(json.Unmarshal(
		resp.body, dst),
		"%s %s: %s",
		resp.method, resp.path, resp.body)
}
