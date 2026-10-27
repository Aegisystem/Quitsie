package httpapi

import "context"

// Operations of openapi.yaml for Groups, Members, invitations and Friendships that are not built yet. Each
// answers 501 not_implemented, so Server keeps satisfying
// StrictServerInterface while the contract is ahead of the code. Implement an
// operation by moving its method out of this file; delete the file when it is empty.

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
