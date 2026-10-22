package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// specPath is the shared API contract, relative to this package.
const specPath = "../../../api/openapi.yaml"

// contractServer runs a handler and checks every request and response
// against the OpenAPI spec, so the backend can't drift from the contract
// the frontend is built against.
type contractServer struct {
	ts      *httptest.Server
	router  routers.Router
	cookies []*http.Cookie // sent with every request
}

func newContractServer(t *testing.T, h http.Handler) *contractServer {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("loading spec: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("invalid spec: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("building router: %v", err)
	}

	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &contractServer{ts: ts, router: router}
}

// do sends a request and fails the test if the request or the response
// doesn't match the spec. The returned response body is fully readable.
func (s *contractServer) do(t *testing.T, method, path string, body []byte) *http.Response {
	t.Helper()
	return s.send(t, method, path, body, true)
}

// doInvalid sends a request that deliberately breaks the spec, to test how
// the server rejects it. The response is still checked against the spec.
func (s *contractServer) doInvalid(t *testing.T, method, path string, body []byte) *http.Response {
	t.Helper()
	return s.send(t, method, path, body, false)
}

func (s *contractServer) send(t *testing.T, method, path string, body []byte, checkRequest bool) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, s.ts.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range s.cookies {
		req.AddCookie(c)
	}

	route, pathParams, err := s.router.FindRoute(req)
	if err != nil {
		t.Fatalf("%s %s is not in the spec: %v", method, path, err)
	}
	reqInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			// Sessions aren't implemented yet; see authenticate in server.go.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			// Fail on status codes the spec doesn't document.
			IncludeResponseStatus: true,
		},
	}
	if checkRequest {
		if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err != nil {
			t.Fatalf("request does not match spec: %v", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	res, err := s.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	resInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 res.StatusCode,
		Header:                 res.Header,
		Body:                   io.NopCloser(bytes.NewReader(resBody)),
		Options:                reqInput.Options,
	}
	if err := openapi3filter.ValidateResponse(context.Background(), resInput); err != nil {
		t.Fatalf("response does not match spec: %v\nbody: %s", err, resBody)
	}

	res.Body = io.NopCloser(bytes.NewReader(resBody))
	return res
}

// logIn logs in through the API and keeps the session cookie for later requests.
func (s *contractServer) logIn(t *testing.T, username, password string) {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"username":%q,"password":%q}`, username, password))
	res := s.do(t, http.MethodPost, "/api/session", body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("logging in as %s: status %d", username, res.StatusCode)
	}
	s.cookies = res.Cookies()
}

// anonymous returns a client for the same server without a session.
func (s *contractServer) anonymous() *contractServer {
	return &contractServer{ts: s.ts, router: s.router}
}
