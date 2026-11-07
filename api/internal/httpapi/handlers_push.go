package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/push"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// errPushOff answers the push operations of a server without VAPID keys.
var errPushOff = &failure{status: http.StatusNotFound, code: ProblemCodeNotFound, title: "This server sends no push notifications."}

func (s *Server) GetPushKey(context.Context, GetPushKeyRequestObject) (GetPushKeyResponseObject, error) {
	if s.deps.PushKey == "" {
		return GetPushKey404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse(errPushOff.problem())}, nil
	}
	return GetPushKey200JSONResponse{PublicKey: s.deps.PushKey}, nil
}

func (s *Server) SavePushSubscription(ctx context.Context, req SavePushSubscriptionRequestObject) (SavePushSubscriptionResponseObject, error) {
	respond := func(status int, p Problem) SavePushSubscriptionResponseObject {
		return SavePushSubscriptiondefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	if s.deps.PushKey == "" {
		return answer(errPushOff, respond)
	}
	session, ok := auth.CurrentSession(ctx)
	if !ok {
		return answer(errUnauthenticated, respond)
	}
	body := req.Body
	if err := push.CheckEndpoint(body.Endpoint); err != nil {
		return answer(invalid("%v", err), respond)
	}
	if err := push.CheckDeviceKeys(body.Keys.P256dh, body.Keys.Auth); err != nil {
		return answer(invalid("%v", err), respond)
	}
	if err := saveSubscription(ctx, s.deps.DB, session.ID, body); err != nil {
		return nil, err
	}
	return SavePushSubscription204Response{}, nil
}

// saveSubscription replaces the subscription of the session with sub. It
// takes the write lock, so two saves of one session never race on its
// single row.
func saveSubscription(ctx context.Context, db *sql.DB, sessionID string, sub *PushSubscription) error {
	tx, err := store.BeginWrite(ctx, db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := store.New(tx)
	if err := q.DeleteOtherPushSubscriptionOfSession(ctx, store.DeleteOtherPushSubscriptionOfSessionParams{SessionID: sessionID, Endpoint: sub.Endpoint}); err != nil {
		return err
	}
	if err := q.SavePushSubscription(ctx, store.SavePushSubscriptionParams{
		Endpoint: sub.Endpoint, SessionID: sessionID, P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth, CreatedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) DeletePushSubscription(ctx context.Context, _ DeletePushSubscriptionRequestObject) (DeletePushSubscriptionResponseObject, error) {
	session, ok := auth.CurrentSession(ctx)
	if !ok {
		return nil, errors.New("a signed-in request without a session")
	}
	if err := store.New(s.deps.DB).DeletePushSubscriptionOfSession(ctx, session.ID); err != nil {
		return nil, err
	}
	return DeletePushSubscription204Response{}, nil
}
