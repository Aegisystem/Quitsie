package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/domain"
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
			authProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "The email address is not valid."))}, nil
	case errors.As(err, &limited):
		seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
		return RequestLoginCode429ApplicationProblemPlusJSONResponse{
			Body:    authProblem(http.StatusTooManyRequests, ProblemCodeRateLimited, "Too many login codes requested."),
			Headers: RequestLoginCode429ResponseHeaders{RetryAfter: &seconds},
		}, nil
	default:
		return nil, err
	}
}

func (s *Server) VerifyLoginCode(ctx context.Context, req VerifyLoginCodeRequestObject) (VerifyLoginCodeResponseObject, error) {
	if !sixDigits.MatchString(req.Body.Code) {
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			authProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "A login code is 6 digits.")), nil
	}
	r := requestFrom(ctx)
	signIn, err := s.deps.Auth.VerifyCode(ctx, string(req.Body.Email), req.Body.Code, device(r))
	switch {
	case err == nil:
	case errors.Is(err, auth.ErrInvalidEmail):
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			authProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "The email address is not valid.")), nil
	case errors.Is(err, auth.ErrWrongCode):
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			authProblem(http.StatusBadRequest, ProblemCodeLoginCodeInvalid, "The login code is wrong.")), nil
	case errors.Is(err, auth.ErrCodeExpired):
		return VerifyLoginCode400ApplicationProblemPlusJSONResponse(
			authProblem(http.StatusBadRequest, ProblemCodeLoginCodeExpired, "The login code expired; request another.")), nil
	case errors.Is(err, auth.ErrTooManyAttempts):
		return VerifyLoginCode429ApplicationProblemPlusJSONResponse(
			authProblem(http.StatusTooManyRequests, ProblemCodeTooManyAttempts, "The login code was guessed wrong too many times; request another.")), nil
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

func (s *Server) StartGoogleLogin(ctx context.Context, req StartGoogleLoginRequestObject) (StartGoogleLoginResponseObject, error) {
	returnTo := "/"
	if req.Params.ReturnTo != nil {
		returnTo = *req.Params.ReturnTo
		if !insideTheApp(returnTo) {
			return StartGoogleLogindefaultApplicationProblemPlusJSONResponse{
				StatusCode: http.StatusBadRequest,
				Body:       authProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "returnTo must be a path of the app."),
			}, nil
		}
	}
	authURL, flow, err := s.deps.Auth.BeginGoogle(ctx, returnTo)
	if err != nil {
		slog.WarnContext(ctx, "cannot start Google sign-in", "err", err)
		return redirect{location: googleFailedPath}, nil
	}
	return redirect{location: authURL, cookies: []*http.Cookie{flow}}, nil
}

// googleFailedPath is the login screen, told to say that Google failed.
const googleFailedPath = "/login?error=google"

func (s *Server) FinishGoogleLogin(ctx context.Context, _ FinishGoogleLoginRequestObject) (FinishGoogleLoginResponseObject, error) {
	r := requestFrom(ctx)
	clearFlow := s.deps.Auth.ClearGoogleCookie()
	signIn, err := s.deps.Auth.FinishGoogle(ctx, r, device(r))
	switch {
	case err == nil:
		returnTo := signIn.ReturnTo
		if !insideTheApp(returnTo) {
			returnTo = "/"
		}
		return redirect{location: returnTo, cookies: []*http.Cookie{clearFlow, auth.SessionCookie(signIn.Token)}}, nil
	case errors.Is(err, auth.ErrGoogleCancelled):
		return redirect{location: "/login", cookies: []*http.Cookie{clearFlow}}, nil
	default:
		slog.WarnContext(ctx, "Google sign-in failed", "err", err)
		return redirect{location: googleFailedPath, cookies: []*http.Cookie{clearFlow}}, nil
	}
}

// redirect sends a browser somewhere else with a 302. The generated
// responses of the Google operations have room for one cookie, and they
// would overwrite the one Authenticate sets when it renews a session.
type redirect struct {
	location string
	cookies  []*http.Cookie
}

func (d redirect) visit(w http.ResponseWriter) error {
	for _, c := range d.cookies {
		http.SetCookie(w, c)
	}
	w.Header().Set("Location", d.location)
	w.WriteHeader(http.StatusFound)
	return nil
}

func (d redirect) VisitStartGoogleLoginResponse(w http.ResponseWriter) error { return d.visit(w) }

func (d redirect) VisitFinishGoogleLoginResponse(w http.ResponseWriter) error { return d.visit(w) }

// maxReturnToLength is the contract's limit on returnTo.
const maxReturnToLength = 512

// insideTheApp reports whether path can be redirected to without leaving
// the app. Browsers read "//host" and "/\host" as another host, and they
// drop tabs and newlines before reading a URL.
func insideTheApp(path string) bool {
	if len(path) > maxReturnToLength || !strings.HasPrefix(path, "/") || strings.ContainsFunc(path, unicode.IsControl) {
		return false
	}
	return !strings.HasPrefix(path, "//") && !strings.HasPrefix(path, `/\`)
}

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
			authProblem(http.StatusNotFound, ProblemCodeNotFound, "There is no such session."))}, nil
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

func (s *Server) DeleteMe(ctx context.Context, _ DeleteMeRequestObject) (DeleteMeResponseObject, error) {
	user, _ := auth.CurrentUser(ctx)
	if err := s.deps.Auth.DeleteUser(ctx, user.ID); err != nil {
		return nil, err
	}
	return DeleteMe204Response{Headers: DeleteMe204ResponseHeaders{SetCookie: clearSessionCookie()}}, nil
}

// maxNameLength is the longest name a User can have, in characters.
const maxNameLength = 80

func (s *Server) UpdateMe(ctx context.Context, req UpdateMeRequestObject) (UpdateMeResponseObject, error) {
	invalid := func(title string) (UpdateMeResponseObject, error) {
		return UpdateMe400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(
			authProblem(http.StatusBadRequest, ProblemCodeValidationFailed, title))}, nil
	}
	change := store.UpdateUserParams{PreferredCurrency: req.Body.PreferredCurrency}
	if req.Body.Name == nil && req.Body.PreferredCurrency == nil {
		return invalid("Send the name, the preferred currency or both.")
	}
	if req.Body.Name != nil {
		name := strings.TrimSpace(*req.Body.Name)
		if name == "" || utf8.RuneCountInString(name) > maxNameLength {
			return invalid("The name must have between 1 and 80 characters.")
		}
		change.Name = &name
	}
	if c := req.Body.PreferredCurrency; c != nil {
		if _, ok := domain.Currency(*c).Decimals(); !ok {
			return invalid("The preferred currency is not an ISO 4217 currency.")
		}
	}

	user, _ := auth.CurrentUser(ctx)
	change.ID = user.ID
	updated, err := store.New(s.deps.DB).UpdateUser(ctx, change)
	if err != nil {
		return nil, err
	}
	return UpdateMe200JSONResponse(userFrom(updated)), nil
}

// authProblem is the body of an error of these operations.
func authProblem(status int, code ProblemCode, title string) Problem {
	return Problem{Status: status, Code: code, Title: title}
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
