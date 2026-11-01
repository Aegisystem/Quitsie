package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Sign-in, sessions and the profile, on top of package auth.

// GetLoginMethods lets the PWA show the Google button only when it can work:
// the owner may run the server without an OAuth client of Google.
func (s *Server) GetLoginMethods(context.Context, GetLoginMethodsRequestObject) (GetLoginMethodsResponseObject, error) {
	return GetLoginMethods200JSONResponse{Google: s.deps.Auth.GoogleEnabled()}, nil
}

func (s *Server) RequestLoginCode(ctx context.Context, req RequestLoginCodeRequestObject) (RequestLoginCodeResponseObject, error) {
	r := requestFrom(ctx)
	flow, err := s.deps.Auth.RequestCode(ctx, string(req.Body.Email), auth.BrowserOf(r, senderIP(r)))
	var limited *auth.RateLimitError
	var spent *auth.MailBudgetError
	switch {
	case err == nil:
		return codeSent{flow: flow}, nil
	case errors.Is(err, auth.ErrInvalidEmail):
		return RequestLoginCode400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(
			authProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "The email address is not valid."))}, nil
	case errors.As(err, &limited):
		seconds := retryAfterSeconds(limited.RetryAfter)
		return RequestLoginCode429ApplicationProblemPlusJSONResponse{
			Body:    authProblem(http.StatusTooManyRequests, ProblemCodeRateLimited, "Too many login codes requested."),
			Headers: RequestLoginCode429ResponseHeaders{RetryAfter: &seconds},
		}, nil
	case errors.As(err, &spent):
		seconds := retryAfterSeconds(spent.RetryAfter)
		return RequestLoginCode503ApplicationProblemPlusJSONResponse{
			Body:    authProblem(http.StatusServiceUnavailable, ProblemCodeMailBudgetSpent, "The server sent all the emails it can for now."),
			Headers: RequestLoginCode503ResponseHeaders{RetryAfter: &seconds},
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
	signIn, err := s.deps.Auth.VerifyCode(ctx, string(req.Body.Email), req.Body.Code, device(r), auth.BrowserOf(r, clientIP(r)))
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
	case errors.Is(err, auth.ErrRateLimited):
		return VerifyLoginCode429ApplicationProblemPlusJSONResponse(
			authProblem(http.StatusTooManyRequests, ProblemCodeRateLimited, "Too many login codes tried.")), nil
	default:
		return nil, err
	}
	return codeChecked{
		body:    LoginResult{User: userFrom(signIn.User), NewUser: signIn.NewUser},
		cookies: []*http.Cookie{auth.SessionCookie(signIn.Token), auth.KnownDeviceCookie(signIn.KnownDevices), auth.ClearCodeFlowCookie()},
	}, nil
}

// codeSent and codeChecked answer with cookies. The generated responses have
// room for one Set-Cookie at most, and they would overwrite the one
// Authenticate sets when it renews a session.
type codeSent struct {
	flow *http.Cookie
}

func (c codeSent) VisitRequestLoginCodeResponse(w http.ResponseWriter) error {
	http.SetCookie(w, c.flow)
	return RequestLoginCode204Response{}.VisitRequestLoginCodeResponse(w)
}

type codeChecked struct {
	body    LoginResult
	cookies []*http.Cookie
}

func (c codeChecked) VisitVerifyLoginCodeResponse(w http.ResponseWriter) error {
	for _, cookie := range c.cookies {
		http.SetCookie(w, cookie)
	}
	return VerifyLoginCode200JSONResponse{Body: c.body}.VisitVerifyLoginCodeResponse(w)
}

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// retryAfterSeconds is the value of a Retry-After header, rounded up so the
// client never asks too early.
func retryAfterSeconds(d time.Duration) int {
	return int(math.Ceil(d.Seconds()))
}

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
		return redirect{location: returnTo, cookies: []*http.Cookie{clearFlow, auth.SessionCookie(signIn.Token), auth.KnownDeviceCookie(signIn.KnownDevices)}}, nil
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
