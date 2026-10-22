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

	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store"
	"family-cookbook/backend/internal/version"

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
	auth  *auth.Service
	now   func() time.Time
	newID func() string
	// release and commit identify the running build; see package version.
	release, commit string
	// insecureCookies drops the Secure flag from cookies, for plain-HTTP setups.
	insecureCookies bool
}

var _ StrictServerInterface = (*server)(nil)

// Options configures NewServer.
type Options struct {
	// InsecureCookies lets the session cookie travel over plain HTTP. It's
	// only needed when the app is reached over HTTP by a name other than
	// localhost; behind an HTTPS reverse proxy, leave it off.
	InsecureCookies bool
}

// NewServer returns the HTTP handler for the whole API, mounted under /api.
func NewServer(st store.Store, a *auth.Service, opts Options) http.Handler {
	return newHandler(&server{
		store:           st,
		auth:            a,
		now:             time.Now,
		newID:           rand.Text,
		release:         version.Release(),
		commit:          version.Commit(),
		insecureCookies: opts.InsecureCookies,
	})
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

	return limitBody(s.authenticate(validateRequests(routes)))
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
			// The spec says which endpoints need a session (all but those with
			// `security: []`); authenticate has already found the user, if any.
			AuthenticationFunc: func(ctx context.Context, _ *openapi3filter.AuthenticationInput) error {
				if currentUser(ctx) == "" {
					return errNotLoggedIn
				}
				return nil
			},
		},
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			if status == http.StatusUnauthorized {
				message = errNotLoggedIn.Error()
			}
			writeError(w, status, message)
		},
	})(next)
}

var errNotLoggedIn = errors.New("not logged in")

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

func (s *server) GetInfo(ctx context.Context, _ GetInfoRequestObject) (GetInfoResponseObject, error) {
	info := GetInfo200JSONResponse{Commit: s.commit}
	if s.release != "" {
		info.Version = &s.release
	}
	return info, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Error{Message: message}); err != nil {
		log.Printf("writing error response: %v", err)
	}
}
