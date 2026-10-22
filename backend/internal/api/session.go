package api

import (
	"context"
	"errors"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store"
)

const sessionCookie = "session"

// request holds what authenticate learned about a request, for the handlers.
type request struct {
	user    string // empty when not logged in
	userID  int64
	session store.Session
	token   string // the session cookie as sent, even if no longer valid
	secure  bool   // whether cookies set in reply get the Secure flag
}

type requestKey struct{}

func requestInfo(ctx context.Context) request {
	r, _ := ctx.Value(requestKey{}).(request)
	return r
}

// currentUser returns the logged-in username, or "" for anonymous requests.
func currentUser(ctx context.Context) string {
	return requestInfo(ctx).user
}

// currentUserID returns the logged-in user's ID, or 0 for anonymous requests.
func currentUserID(ctx context.Context) int64 {
	return requestInfo(ctx).userID
}

// authenticate attaches the session's user to the request, if the session
// cookie is valid. It never rejects requests: the validator does that for
// endpoints the spec says need a session.
func (s *server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := request{secure: !s.insecureCookies && !isLocalhost(r.Host)}
		if c, err := r.Cookie(sessionCookie); err == nil {
			info.token = c.Value
			sess, renewed, err := s.auth.Authenticate(r.Context(), c.Value)
			switch {
			case err == nil:
				info.user, info.userID, info.session = sess.Username, sess.UserID, sess
				if renewed {
					http.SetCookie(w, s.sessionCookie(c.Value, info.secure))
				}
			case !errors.Is(err, auth.ErrNoSession):
				log.Printf("checking session: %v", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey{}, info)))
	})
}

func (s *server) sessionCookie(token string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// isLocalhost reports whether host (from the Host header) is this machine.
// Browsers treat http://localhost as secure, so the Secure flag isn't needed
// there, and leaving it off keeps local development over plain HTTP working.
func isLocalhost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *server) LogIn(ctx context.Context, req LogInRequestObject) (LogInResponseObject, error) {
	token, sess, err := s.auth.LogIn(ctx, req.Body.Username, req.Body.Password)
	var tooMany *auth.TooManyAttemptsError
	switch {
	case errors.As(err, &tooMany):
		seconds := int(math.Ceil(tooMany.RetryAfter.Seconds()))
		return LogIn429JSONResponse{TooManyRequestsJSONResponse{
			Body:    Error{Message: "too many failed attempts; try again later"},
			Headers: TooManyRequestsResponseHeaders{RetryAfter: &seconds},
		}}, nil
	case errors.Is(err, auth.ErrInvalidCredentials):
		return LogIn401JSONResponse{Message: "wrong username or password"}, nil
	case err != nil:
		return nil, err
	}
	cookie := s.sessionCookie(token, requestInfo(ctx).secure).String()
	return LogIn200JSONResponse{
		Body:    User{Username: sess.Username},
		Headers: LogIn200ResponseHeaders{SetCookie: &cookie},
	}, nil
}

func (s *server) LogOut(ctx context.Context, _ LogOutRequestObject) (LogOutResponseObject, error) {
	info := requestInfo(ctx)
	if err := s.auth.LogOut(ctx, info.token); err != nil {
		return nil, err
	}
	expired := s.sessionCookie("", info.secure)
	expired.MaxAge = -1
	expired.Expires = time.Unix(0, 0)
	cookie := expired.String()
	return LogOut204Response{Headers: LogOut204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (s *server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	return GetMe200JSONResponse{Username: currentUser(ctx)}, nil
}

func (s *server) SetUpPassword(ctx context.Context, req SetUpPasswordRequestObject) (SetUpPasswordResponseObject, error) {
	token, sess, err := s.auth.CompleteSetup(ctx, req.Body.Token, req.Body.Password)
	var bad *auth.BadPasswordError
	switch {
	case errors.As(err, &bad):
		return SetUpPassword400JSONResponse{Message: bad.Reason}, nil
	case errors.Is(err, auth.ErrInvalidLink):
		return SetUpPassword400JSONResponse{Message: auth.ErrInvalidLink.Error()}, nil
	case err != nil:
		return nil, err
	}
	cookie := s.sessionCookie(token, requestInfo(ctx).secure).String()
	return SetUpPassword200JSONResponse{
		Body:    User{Username: sess.Username},
		Headers: SetUpPassword200ResponseHeaders{SetCookie: &cookie},
	}, nil
}

func (s *server) ChangePassword(ctx context.Context, req ChangePasswordRequestObject) (ChangePasswordResponseObject, error) {
	err := s.auth.ChangePassword(ctx, requestInfo(ctx).session, req.Body.CurrentPassword, req.Body.NewPassword)
	var bad *auth.BadPasswordError
	var tooMany *auth.TooManyAttemptsError
	switch {
	case errors.As(err, &tooMany):
		seconds := int(math.Ceil(tooMany.RetryAfter.Seconds()))
		return ChangePassword429JSONResponse{TooManyRequestsJSONResponse{
			Body:    Error{Message: "too many failed attempts; try again later"},
			Headers: TooManyRequestsResponseHeaders{RetryAfter: &seconds},
		}}, nil
	case errors.Is(err, auth.ErrInvalidCredentials):
		// 400, not 401: the session is fine, only the password given is wrong.
		return ChangePassword400JSONResponse{Message: "the current password is wrong"}, nil
	case errors.As(err, &bad):
		return ChangePassword400JSONResponse{Message: bad.Reason}, nil
	case err != nil:
		return nil, err
	}
	return ChangePassword204Response{}, nil
}
