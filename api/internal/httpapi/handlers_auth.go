package httpapi

import (
	"context"
	"net"
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Sign-in, sessions and the profile, on top of package auth.

func (s *Server) RequestLoginCode(ctx context.Context, req RequestLoginCodeRequestObject) (RequestLoginCodeResponseObject, error) {
	r := requestFrom(ctx)
	if err := s.deps.Auth.RequestCode(ctx, string(req.Body.Email), clientIP(r)); err != nil {
		return nil, err
	}
	return RequestLoginCode204Response{}, nil
}

func (s *Server) VerifyLoginCode(ctx context.Context, req VerifyLoginCodeRequestObject) (VerifyLoginCodeResponseObject, error) {
	r := requestFrom(ctx)
	signIn, err := s.deps.Auth.VerifyCode(ctx, string(req.Body.Email), req.Body.Code, r.UserAgent())
	if err != nil {
		return nil, err
	}
	cookie := auth.SessionCookie(signIn.Token).String()
	return VerifyLoginCode200JSONResponse{
		Body:    LoginResult{User: userFrom(signIn.User), NewUser: signIn.NewUser},
		Headers: VerifyLoginCode200ResponseHeaders{SetCookie: &cookie},
	}, nil
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
