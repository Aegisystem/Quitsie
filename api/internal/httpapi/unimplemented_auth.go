package httpapi

import "context"

// Operations of openapi.yaml for sign-in, sessions and the profile that are not built yet. Each
// answers 501 not_implemented, so Server keeps satisfying
// StrictServerInterface while the contract is ahead of the code. Implement an
// operation by moving its method out of this file; delete the file when it is empty.

// --- auth ---

func (s *Server) RequestLoginCode(context.Context, RequestLoginCodeRequestObject) (RequestLoginCodeResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) VerifyLoginCode(context.Context, VerifyLoginCodeRequestObject) (VerifyLoginCodeResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) StartGoogleLogin(context.Context, StartGoogleLoginRequestObject) (StartGoogleLoginResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) FinishGoogleLogin(context.Context, FinishGoogleLoginRequestObject) (FinishGoogleLoginResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) Logout(context.Context, LogoutRequestObject) (LogoutResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) ListSessions(context.Context, ListSessionsRequestObject) (ListSessionsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RevokeSession(context.Context, RevokeSessionRequestObject) (RevokeSessionResponseObject, error) {
	return nil, errNotImplemented
}

// --- profile ---

func (s *Server) GetMe(context.Context, GetMeRequestObject) (GetMeResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UpdateMe(context.Context, UpdateMeRequestObject) (UpdateMeResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) DeleteMe(context.Context, DeleteMeRequestObject) (DeleteMeResponseObject, error) {
	return nil, errNotImplemented
}
