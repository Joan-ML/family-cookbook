// Package api implements the HTTP API described in api/openapi.yaml.
//
// Types and routing are generated into api.gen.go (run `go generate ./...`);
// this package provides the handlers behind the generated StrictServerInterface.
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"family-cookbook/backend/internal/store"

	"github.com/getkin/kin-openapi/openapi3filter"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

const (
	basePath = "/api"
	// maxBodyBytes caps JSON request bodies; photos will use their own endpoint.
	maxBodyBytes = 1 << 20
)

// server implements StrictServerInterface.
type server struct {
	store store.Store
	now   func() time.Time
	newID func() string
}

var _ StrictServerInterface = (*server)(nil)

// NewServer returns the HTTP handler for the whole API, mounted under /api.
func NewServer(st store.Store) http.Handler {
	return newHandler(&server{store: st, now: time.Now, newID: rand.Text})
}

func newHandler(s *server) http.Handler {
	strict := NewStrictHandlerWithOptions(s, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		},
	})
	routes := HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL: basePath,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err.Error())
		},
	})

	return limitBody(validateRequests(authenticate(routes)))
}

// validateRequests rejects requests that don't match the spec (unknown
// routes, missing fields, values out of range) before they reach a handler.
func validateRequests(next http.Handler) http.Handler {
	spec, err := GetSwagger()
	if err != nil {
		panic(err) // the embedded spec is generated, so this is a build bug
	}
	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		DoNotValidateServers: true,
		Prefix:               basePath,
		Options: openapi3filter.Options{
			// Sessions are checked by authenticate, not by the validator.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			writeError(w, status, message)
		},
	})(next)
}

type userKey struct{}

// devUser is attached to every request until login exists.
const devUser = "dev"

// authenticate attaches the current user to the request context. It is a
// placeholder: once login exists, it will read the session cookie and
// reply 401 when there is no valid session.
func authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userKey{}, devUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// currentUser returns the username attached by authenticate.
func currentUser(ctx context.Context) string {
	user, _ := ctx.Value(userKey{}).(string)
	return user
}

// limitBody rejects bodies over maxBodyBytes with 413. It reads the body up
// front: otherwise the validator, which reads it first, reports the overflow
// as an unrelated validation error.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "reading request body: "+err.Error())
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func (s *server) GetHealth(ctx context.Context, _ GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{Status: Ok}, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Error{Message: message}); err != nil {
		log.Printf("writing error response: %v", err)
	}
}
