package httpapi

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Sign-in, sessions and the profile, on top of package auth.

func (s *Server) RequestLoginCode(ctx context.Context, req RequestLoginCodeRequestObject) (RequestLoginCodeResponseObject, error) {
	r := requestFrom(ctx)
	err := s.deps.Auth.RequestCode(ctx, string(req.Body.Email), clientIP(r))
	var limited *auth.RateLimitError
	switch {
	case err == nil:
		return RequestLoginCode204Response{}, nil
	case errors.Is(err, auth.ErrInvalidEmail):
		return RequestLoginCode400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(
			problem(http.StatusBadRequest, ProblemCodeValidationFailed, "The email address is not valid."))}, nil
	case errors.As(err, &limited):
		seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
		return RequestLoginCode429ApplicationProblemPlusJSONResponse{
			Body:    problem(http.StatusTooManyRequests, ProblemCodeRateLimited, "Too many login codes requested."),
			Headers: RequestLoginCode429ResponseHeaders{RetryAfter: &seconds},
		}, nil
	default:
		return nil, err
	}
}

func (s *Server) VerifyLoginCode(ctx context.Context, req VerifyLoginCodeRequestObject) (VerifyLoginCodeResponseObject, error) {
	if !sixDigits.MatchString(req.Body.Code) {
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			problem(http.StatusBadRequest, ProblemCodeValidationFailed, "A login code is 6 digits.")), nil
	}
	r := requestFrom(ctx)
	signIn, err := s.deps.Auth.VerifyCode(ctx, string(req.Body.Email), req.Body.Code, device(r))
	switch {
	case err == nil:
	case errors.Is(err, auth.ErrInvalidEmail):
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			problem(http.StatusBadRequest, ProblemCodeValidationFailed, "The email address is not valid.")), nil
	case errors.Is(err, auth.ErrWrongCode):
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			problem(http.StatusBadRequest, ProblemCodeLoginCodeInvalid, "The login code is wrong.")), nil
	case errors.Is(err, auth.ErrCodeExpired):
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			problem(http.StatusBadRequest, ProblemCodeLoginCodeExpired, "The login code expired; request another.")), nil
	case errors.Is(err, auth.ErrTooManyAttempts):
		return VerifyLoginCode429ApplicationProblemPlusJSONResponse(
			problem(http.StatusTooManyRequests, ProblemCodeTooManyAttempts, "The login code was guessed wrong too many times; request another.")), nil
	default:
		return nil, err
	}
	cookie := auth.SessionCookie(signIn.Token).String()
	return VerifyLoginCode200JSONResponse{
		Body:    LoginResult{User: userFrom(signIn.User), NewUser: signIn.NewUser},
		Headers: VerifyLoginCode200ResponseHeaders{SetCookie: &cookie},
	}, nil
}

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

func (s *Server) Logout(ctx context.Context, _ LogoutRequestObject) (LogoutResponseObject, error) {
	user, _ := auth.CurrentUser(ctx)
	session, _ := auth.CurrentSession(ctx)
	// Already ended, say on another device: signing out is done either way.
	if err := s.deps.Auth.EndSession(ctx, user.ID, session.ID); err != nil && !errors.Is(err, auth.ErrSessionNotFound) {
		return nil, err
	}
	return Logout204Response{Headers: Logout204ResponseHeaders{SetCookie: clearSessionCookie()}}, nil
}

func (s *Server) ListSessions(ctx context.Context, _ ListSessionsRequestObject) (ListSessionsResponseObject, error) {
	user, _ := auth.CurrentUser(ctx)
	current, _ := auth.CurrentSession(ctx)
	sessions, err := s.deps.Auth.Sessions(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	list := make(ListSessions200JSONResponse, len(sessions))
	for i, session := range sessions {
		list[i] = Session{
			Id:         uuid.MustParse(session.ID),
			Device:     session.Device,
			CreatedAt:  session.CreatedAt,
			LastUsedAt: session.LastUsedAt,
			Current:    session.ID == current.ID,
		}
	}
	return list, nil
}

func (s *Server) RevokeSession(ctx context.Context, req RevokeSessionRequestObject) (RevokeSessionResponseObject, error) {
	user, _ := auth.CurrentUser(ctx)
	err := s.deps.Auth.EndSession(ctx, user.ID, req.SessionId.String())
	if errors.Is(err, auth.ErrSessionNotFound) {
		return RevokeSession404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse(
			problem(http.StatusNotFound, ProblemCodeNotFound, "There is no such session."))}, nil
	}
	if err != nil {
		return nil, err
	}
	// Closing the current session signs this device out: its next request
	// finds no session and deletes the cookie.
	return RevokeSession204Response{}, nil
}

// clearSessionCookie is the Set-Cookie header that signs the browser out.
func clearSessionCookie() *string {
	c := auth.ClearSessionCookie().String()
	return &c
}

func (s *Server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	user, _ := auth.CurrentUser(ctx) // requireSession checked there is one
	return GetMe200JSONResponse(userFrom(user)), nil
}

func userFrom(u store.User) User {
	return User{
		Id:                uuid.MustParse(u.ID),
		Email:             openapi_types.Email(u.Email),
		Name:              u.Name,
		PreferredCurrency: u.PreferredCurrency,
	}
}

// maxDeviceLength bounds what a session keeps of the User-Agent, which the
// client controls.
const maxDeviceLength = 256

// device describes the device of a sign-in for the list of sessions. It is
// the User-Agent; the PWA turns it into something like "Safari en iPhone".
func device(r *http.Request) string {
	ua := r.UserAgent()
	if len(ua) > maxDeviceLength {
		ua = strings.ToValidUTF8(ua[:maxDeviceLength], "")
	}
	return ua
}

// clientIP is the address the request came from. Behind cloudflared every
// request comes from 127.0.0.1, and Cloudflare puts the real one in
// CF-Connecting-IP; the server only listens on loopback, so nobody else
// can set that header.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
