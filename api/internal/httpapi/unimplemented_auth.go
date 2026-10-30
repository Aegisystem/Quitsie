package httpapi

import "context"

// Operations of openapi.yaml for sign-in, sessions and the profile that are not built yet. Each
// answers 501 not_implemented, so Server keeps satisfying
// StrictServerInterface while the contract is ahead of the code. Implement an
// operation by moving its method out of this file; delete the file when it is empty.

// --- auth ---

func (s *Server) StartGoogleLogin(context.Context, StartGoogleLoginRequestObject) (StartGoogleLoginResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) FinishGoogleLogin(context.Context, FinishGoogleLoginRequestObject) (FinishGoogleLoginResponseObject, error) {
	return nil, errNotImplemented
}

// --- profile ---
