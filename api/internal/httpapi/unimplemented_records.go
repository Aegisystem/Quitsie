package httpapi

import "context"

// Operations of openapi.yaml for Expenses, Settlements, records and sync that are not built yet. Each
// answers 501 not_implemented, so Server keeps satisfying
// StrictServerInterface while the contract is ahead of the code. Implement an
// operation by moving its method out of this file; delete the file when it is empty.

// --- records ---

func (s *Server) DeleteSettlement(context.Context, DeleteSettlementRequestObject) (DeleteSettlementResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) RestoreSettlement(context.Context, RestoreSettlementRequestObject) (RestoreSettlementResponseObject, error) {
	return nil, errNotImplemented
}
