package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// heartbeat keeps an idle event stream open through proxies that close silent
// connections, and lets the server notice a device that went away.
const heartbeat = 25 * time.Second

// reconnectAfter is how long the browser waits before opening the stream
// again after it drops.
const reconnectAfter = 5 * time.Second

// streams ends every open event stream when the server shuts down, since
// they never end on their own and would hold up the graceful shutdown.
type streams struct {
	made, closed sync.Once
	done         chan struct{}
}

func (s *streams) closing() chan struct{} {
	s.made.Do(func() { s.done = make(chan struct{}) })
	return s.done
}

// CloseStreams ends the open event streams. cmd/server calls it when it
// starts shutting down; the browsers reconnect to whichever server is up.
func (s *Server) CloseStreams() {
	done := s.streams.closing()
	s.streams.closed.Do(func() { close(done) })
}

func (s *Server) StreamEvents(ctx context.Context, _ StreamEventsRequestObject) (StreamEventsResponseObject, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if s.deps.Changes == nil {
		return nil, errors.New("no change feed")
	}
	var visible map[string]bool
	if err := s.read(ctx, func(q *store.Queries) error {
		ledgers, err := q.ListLedgersOfUser(ctx, user.ID)
		visible = make(map[string]bool, len(ledgers))
		for _, ledger := range ledgers {
			visible[ledger.ID] = true
		}
		return err
	}); err != nil {
		return nil, err
	}
	return eventStream{ctx: ctx, server: s, userID: user.ID, visible: visible}, nil
}

// eventStream writes a `change` event for each Change of a ledger the User
// sees, until the device goes away or the server shuts down (ADR 0013).
type eventStream struct {
	ctx     context.Context
	server  *Server
	userID  string
	visible map[string]bool
}

func (e eventStream) VisitStreamEventsResponse(w http.ResponseWriter) error {
	changes, cancel := e.server.deps.Changes.Subscribe()
	defer cancel()

	control := http.NewResponseController(w)
	// The server's WriteTimeout is for ordinary responses; a stream lasts as
	// long as the device keeps it open.
	if err := control.SetWriteDeadline(time.Time{}); err != nil {
		return fmt.Errorf("event stream: %w", err)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "retry: %d\n\n", reconnectAfter.Milliseconds()); err != nil {
		return nil
	}
	if err := control.Flush(); err != nil {
		return nil
	}

	beat := time.NewTicker(heartbeat)
	defer beat.Stop()
	for {
		var message string
		select {
		case <-e.ctx.Done():
			return nil
		case <-e.server.streams.closing():
			return nil
		case <-beat.C:
			message = ": ping\n\n"
		case change := <-changes:
			if !e.sees(change) {
				continue
			}
			data, err := json.Marshal(change)
			if err != nil {
				return err
			}
			message = "event: change\ndata: " + string(data) + "\n\n"
		}
		// A write or flush that fails means the device went away.
		if _, err := fmt.Fprint(w, message); err != nil {
			return nil
		}
		if err := control.Flush(); err != nil {
			return nil
		}
	}
}

// sees reports whether the User should hear about change: a ledger they had
// when the stream opened, every ledger after a reconnection of the feed, or a
// ledger they have just joined or claimed. A ledger they lost still gets its
// event, and the pull drops it from the device.
func (e eventStream) sees(change changefeed.Change) bool {
	if change.AnyLedger() || e.visible[change.LedgerID] {
		return true
	}
	var member bool
	err := e.server.read(e.ctx, func(q *store.Queries) error {
		_, err := q.GetActiveMemberOfUser(e.ctx, store.GetActiveMemberOfUserParams{LedgerID: change.LedgerID, UserID: e.userID})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		member = err == nil
		return err
	})
	if err != nil || !member {
		return false
	}
	e.visible[change.LedgerID] = true
	return true
}
