package httpapi

import "context"

// Operations of openapi.yaml that are not built yet. Each answers
// 501 not_implemented, so Server keeps satisfying StrictServerInterface while
// the contract is ahead of the code. Implement an operation by moving its
// method out of this file.

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

// --- groups ---

func (s *Server) CreateGroup(context.Context, CreateGroupRequestObject) (CreateGroupResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UpdateGroup(context.Context, UpdateGroupRequestObject) (UpdateGroupResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) DeleteGroup(context.Context, DeleteGroupRequestObject) (DeleteGroupResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) AddPendingMember(context.Context, AddPendingMemberRequestObject) (AddPendingMemberResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RemoveMember(context.Context, RemoveMemberRequestObject) (RemoveMemberResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UndoClaim(context.Context, UndoClaimRequestObject) (UndoClaimResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) LeaveGroup(context.Context, LeaveGroupRequestObject) (LeaveGroupResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RegenerateGroupInviteLink(context.Context, RegenerateGroupInviteLinkRequestObject) (RegenerateGroupInviteLinkResponseObject, error) {
	return nil, errNotImplemented
}

// --- invites ---

func (s *Server) GetInvite(context.Context, GetInviteRequestObject) (GetInviteResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) ClaimMember(context.Context, ClaimMemberRequestObject) (ClaimMemberResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) JoinGroup(context.Context, JoinGroupRequestObject) (JoinGroupResponseObject, error) {
	return nil, errNotImplemented
}

// --- friendships ---

func (s *Server) CreateFriendship(context.Context, CreateFriendshipRequestObject) (CreateFriendshipResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RegenerateFriendInviteLink(context.Context, RegenerateFriendInviteLinkRequestObject) (RegenerateFriendInviteLinkResponseObject, error) {
	return nil, errNotImplemented
}

// --- records ---

func (s *Server) PushRecords(context.Context, PushRecordsRequestObject) (PushRecordsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UpdateExpense(context.Context, UpdateExpenseRequestObject) (UpdateExpenseResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) DeleteExpense(context.Context, DeleteExpenseRequestObject) (DeleteExpenseResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RestoreExpense(context.Context, RestoreExpenseRequestObject) (RestoreExpenseResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) UpdateSettlement(context.Context, UpdateSettlementRequestObject) (UpdateSettlementResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) DeleteSettlement(context.Context, DeleteSettlementRequestObject) (DeleteSettlementResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RestoreSettlement(context.Context, RestoreSettlementRequestObject) (RestoreSettlementResponseObject, error) {
	return nil, errNotImplemented
}

// --- sync ---

func (s *Server) PullChanges(context.Context, PullChangesRequestObject) (PullChangesResponseObject, error) {
	return nil, errNotImplemented
}
