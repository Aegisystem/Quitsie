package httpapi_test

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/internal/storetest"
)

// eventsAPI is recordsAPI with a change feed, served over a real connection,
// since a stream cannot go through a ResponseRecorder.
func eventsAPI(t *testing.T) (*recordsAPI, *httptest.Server, *httpapi.Server) {
	t.Helper()
	db := storetest.Open(t)
	a := recordsAPIOn(t, db)
	feed := changefeed.New(storetest.URLOf(t, db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	connected, cancel := feed.Subscribe()
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	go feed.Run(ctx)
	select {
	case <-connected:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("the change feed did not connect")
	}
	server := httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{DB: db, Auth: a.auth, Origins: []string{testOrigin}, Changes: feed})
	ts := httptest.NewServer(httpapi.NewHandler(server, builtPWA))
	t.Cleanup(ts.Close)
	return a, ts, server
}

// event is one Server-Sent Event: its name and its data.
type event struct{ name, data string }

// openEvents opens p's stream and returns its events as they arrive.
func openEvents(t *testing.T, ts *httptest.Server, p signedIn) (<-chan event, *http.Response) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.token != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: p.token})
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	events := make(chan event, 16)
	go func() {
		defer close(events)
		lines := bufio.NewScanner(resp.Body)
		var e event
		for lines.Scan() {
			line := lines.Text()
			switch {
			case line == "":
				if e.name != "" {
					events <- e
				}
				e = event{}
			case strings.HasPrefix(line, "event: "):
				e.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				e.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return events, resp
}

func nextEvent(t *testing.T, events <-chan event) event {
	t.Helper()
	select {
	case e, ok := <-events:
		if !ok {
			t.Fatal("the stream ended")
		}
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("no event arrived")
		return event{}
	}
}

func noEvent(t *testing.T, events <-chan event) {
	t.Helper()
	select {
	case e, ok := <-events:
		if ok {
			t.Fatalf("unexpected event %+v", e)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

func TestAnOpenDeviceHearsAboutAnExpenseSomeoneElseAdds(t *testing.T) {
	a, ts, _ := eventsAPI(t)
	tr := a.seedTrip()
	anaEvents, resp := openEvents(t, ts, tr.ana)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	evaEvents, _ := openEvents(t, ts, tr.eva)

	a.push(t, tr.beto, httpapi.RecordBatch{Expenses: []httpapi.NewExpense{tr.dinner()}, Settlements: []httpapi.NewSettlement{}})

	e := nextEvent(t, anaEvents)
	if e.name != "change" || !strings.Contains(e.data, `"ledgerId":"`+tr.id+`"`) {
		t.Errorf("event = %+v, want a change of %s", e, tr.id)
	}
	// Eva is in no Member of the trip.
	noEvent(t, evaEvents)
}

func TestADeviceHearsAboutALedgerItJoinsWhileOpen(t *testing.T) {
	a, ts, _ := eventsAPI(t)
	eva := a.signIn("eva@example.com")
	events, _ := openEvents(t, ts, eva)

	// The seeded trip links Ana and Beto, not Eva.
	tr := a.seedTripOf(a.signIn("ana@example.com"), a.signIn("beto@example.com"), eva)
	noEvent(t, events)

	a.write(func(q *store.Queries, version int64) error {
		return q.CreateMember(context.Background(), store.CreateMemberParams{
			ID: uuid.NewString(), LedgerID: tr.id, UserID: &eva.user.ID, Name: "Eva", Version: version, CreatedAt: time.Now(),
		})
	})

	if e := nextEvent(t, events); !strings.Contains(e.data, tr.id) {
		t.Errorf("event = %+v, want a change of %s", e, tr.id)
	}
}

func TestTheStreamNeedsASession(t *testing.T) {
	_, ts, _ := eventsAPI(t)
	resp, err := ts.Client().Get(ts.URL + "/api/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestShuttingDownEndsTheOpenStreams(t *testing.T) {
	a, ts, server := eventsAPI(t)
	events, _ := openEvents(t, ts, a.signIn("ana@example.com"))

	server.CloseStreams()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("got an event instead of the end of the stream")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream stayed open")
	}
}
