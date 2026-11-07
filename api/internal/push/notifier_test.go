package push_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
	"github.com/Aegisystem/Quitsie/api/internal/push"
	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/internal/storetest"
	"github.com/Aegisystem/Quitsie/api/internal/sync"
)

// sent is one notification a device got.
type sent struct {
	endpoint string
	payload  push.Payload
}

// fakeSender keeps what it sends, and answers ErrGone for the endpoints in gone.
type fakeSender struct {
	mu   gosync.Mutex
	sent []sent
	gone map[string]bool
}

func (f *fakeSender) Send(_ context.Context, to push.Subscription, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gone[to.Endpoint] {
		return push.ErrGone
	}
	var p push.Payload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	f.sent = append(f.sent, sent{to.Endpoint, p})
	return nil
}

// take returns what was sent since the last call.
func (f *fakeSender) take() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.sent
	f.sent = nil
	return s
}

// world is a Group, Viaje a Cartagena, of Ana, Beto and Carla, whose devices
// all have notifications on, and the notifier of a server.
type world struct {
	t      *testing.T
	db     *sql.DB
	sender *fakeSender
	now    time.Time
	n      *push.Notifier

	trip                            string
	ana, beto, carla                string // Users
	anaM, betoM, carlaM             string // their Members
	anaPhone, betoPhone, carlaPhone string // their endpoints
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, db: storetest.Open(t), sender: &fakeSender{gone: map[string]bool{}}, now: time.Now()}
	w.n = w.notifier()
	w.ana, w.beto, w.carla = w.user("Ana"), w.user("Beto"), w.user("Carla")
	w.trip = uuid.NewString()
	w.anaM, w.betoM, w.carlaM = uuid.NewString(), uuid.NewString(), uuid.NewString()
	usd := "USD"
	w.write(func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(context.Background(), store.CreateLedgerParams{
			ID: w.trip, Kind: "group", Name: "Viaje a Cartagena", Currency: &usd, SimplifyDebts: true, Version: version, CreatedAt: time.Now(),
		}); err != nil {
			return err
		}
		for _, m := range []struct{ id, user, name string }{{w.anaM, w.ana, "Ana"}, {w.betoM, w.beto, "Beto"}, {w.carlaM, w.carla, "Carla"}} {
			if err := q.CreateMember(context.Background(), store.CreateMemberParams{
				ID: m.id, LedgerID: w.trip, UserID: &m.user, Name: m.name, Version: version, CreatedAt: time.Now(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	w.anaPhone = w.subscribe(w.ana, time.Now())
	w.betoPhone = w.subscribe(w.beto, time.Now())
	w.carlaPhone = w.subscribe(w.carla, time.Now())
	return w
}

func (w *world) notifier() *push.Notifier {
	return push.New(push.Config{DB: w.db, Sender: w.sender, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return w.now }})
}

func (w *world) write(fn func(q *store.Queries, version int64) error) {
	w.t.Helper()
	if _, err := store.Write(context.Background(), w.db, fn); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) user(name string) string {
	w.t.Helper()
	id := uuid.NewString()
	err := store.New(w.db).CreateUser(context.Background(), store.CreateUserParams{
		ID: id, Email: strings.ToLower(name) + "@example.com", Name: name, PreferredCurrency: "USD", CreatedAt: time.Now(),
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

// subscribe opens a session of user, last used at used, with notifications
// on, and returns its endpoint.
func (w *world) subscribe(user string, used time.Time) string {
	w.t.Helper()
	ctx := context.Background()
	q := store.New(w.db)
	session := uuid.NewString()
	if err := q.CreateSession(ctx, store.CreateSessionParams{
		ID: session, UserID: user, TokenHash: uuid.NewString(), Device: "Safari en iPhone", CreatedAt: used, LastUsedAt: used,
	}); err != nil {
		w.t.Fatal(err)
	}
	endpoint := "https://web.push.apple.com/" + uuid.NewString()
	if err := q.SavePushSubscription(ctx, store.SavePushSubscriptionParams{
		Endpoint: endpoint, SessionID: session, P256dh: "key", Auth: "secret", CreatedAt: used,
	}); err != nil {
		w.t.Fatal(err)
	}
	return endpoint
}

// expense has by add an Expense of total that paidBy paid and that owing
// owe in equal parts, and returns its id.
func (w *world) expense(by, description string, total int64, paidBy string, owing ...string) string {
	w.t.Helper()
	e := store.ExpenseWithShares{
		Expense: store.Expense{
			ID: uuid.Must(uuid.NewV7()).String(), LedgerID: w.trip, Description: description, Date: time.Now().UTC().Truncate(24 * time.Hour),
			Currency: "USD", Total: total, SplitMethod: "equal",
		},
		PaidShares: []store.PaidShare{{MemberID: paidBy, Amount: total}},
	}
	share := total / int64(len(owing))
	for i, m := range owing {
		amount := share
		if i == 0 {
			amount += total - share*int64(len(owing))
		}
		e.OwedShares = append(e.OwedShares, store.OwedShare{MemberID: m, Amount: amount})
	}
	w.store(by, sync.Batch{Expenses: []store.ExpenseWithShares{e}})
	return e.ID
}

func (w *world) settlement(by, from, to string, amount int64) {
	w.t.Helper()
	w.store(by, sync.Batch{Settlements: []store.Settlement{{
		ID: uuid.Must(uuid.NewV7()).String(), LedgerID: w.trip, FromMemberID: from, ToMemberID: to,
		Amount: amount, Currency: "USD", Date: time.Now().UTC().Truncate(24 * time.Hour),
	}}})
}

func (w *world) store(by string, batch sync.Batch) {
	w.t.Helper()
	results, err := sync.Push(context.Background(), w.db, by, batch)
	if err != nil {
		w.t.Fatal(err)
	}
	for _, r := range results {
		if r.Rejection != nil {
			w.t.Fatalf("rejected: %v", r.Rejection)
		}
	}
}

// collect has the notifier read what changed, as the change feed would make it.
func (w *world) collect() {
	w.t.Helper()
	if err := w.n.Collect(context.Background()); err != nil {
		w.t.Fatal(err)
	}
}

// after moves the clock by d and sends what is due.
func (w *world) after(d time.Duration) []sent {
	w.now = w.now.Add(d)
	w.n.SendDue(context.Background())
	return w.sender.take()
}

func only(t *testing.T, got []sent, endpoint string) push.Payload {
	t.Helper()
	if len(got) != 1 || got[0].endpoint != endpoint {
		t.Fatalf("sent %+v, want one notification to %s", got, endpoint)
	}
	return got[0].payload
}

func TestAnExpenseReachesTheOtherParticipantsButNotItsWriter(t *testing.T) {
	w := newWorld(t)
	// Carla takes no part in the dinner.
	w.expense(w.beto, "Cena", 9001, w.betoM, w.anaM, w.betoM)
	w.collect()

	if got := w.after(push.Wait - time.Second); len(got) != 0 {
		t.Fatalf("sent %+v before the wait was over", got)
	}
	p := only(t, w.after(time.Second), w.anaPhone)
	if p.LedgerID != w.trip || p.Title != "Viaje a Cartagena" || len(p.Items) != 1 {
		t.Fatalf("payload = %+v", p)
	}
	item := p.Items[0]
	if item.Kind != "expense" || item.Actor != "Beto" || item.Description != "Cena" || item.Amount != 9001 || item.Currency != "USD" {
		t.Errorf("item = %+v", item)
	}
	// The odd cent goes to the first who owes.
	if item.Share == nil || *item.Share != 4501 {
		t.Errorf("share = %v, want 4501", item.Share)
	}
}

func TestASettlementReachesTheMembersItInvolves(t *testing.T) {
	w := newWorld(t)
	w.settlement(w.ana, w.anaM, w.betoM, 2000)
	w.collect()
	item := only(t, w.after(push.Wait), w.betoPhone).Items[0]
	if item.Kind != "settlement" || item.Actor != "Ana" || item.You != "received" || item.Counterpart != "Ana" || !item.ByCounterpart || item.Amount != 2000 {
		t.Errorf("item = %+v", item)
	}

	// Carla writes down that Ana paid Beto: both hear of it, from Carla.
	w.settlement(w.carla, w.anaM, w.betoM, 500)
	w.collect()
	got := w.after(push.Spacing)
	if len(got) != 2 {
		t.Fatalf("sent %+v, want one to Ana and one to Beto", got)
	}
	for _, s := range got {
		item := s.payload.Items[0]
		want := map[string]struct{ you, counterpart string }{w.anaPhone: {"paid", "Beto"}, w.betoPhone: {"received", "Ana"}}[s.endpoint]
		if item.Actor != "Carla" || item.You != want.you || item.Counterpart != want.counterpart || item.ByCounterpart {
			t.Errorf("to %s: item = %+v", s.endpoint, item)
		}
	}
}

func TestSeveralRecordsInARowArriveAsOneNotification(t *testing.T) {
	w := newWorld(t)
	w.expense(w.beto, "Taxi", 3000, w.betoM, w.anaM, w.betoM)
	w.collect()
	w.now = w.now.Add(2 * time.Second)
	w.expense(w.beto, "Museo", 4000, w.betoM, w.anaM, w.betoM)
	w.settlement(w.beto, w.betoM, w.anaM, 100)
	w.collect()

	p := only(t, w.after(push.Wait), w.anaPhone)
	if len(p.Items) != 3 || p.Items[0].Description != "Taxi" || p.Items[1].Description != "Museo" || p.Items[2].Kind != "settlement" {
		t.Fatalf("items = %+v", p.Items)
	}

	// The next one waits until a minute after the last.
	w.expense(w.beto, "Almuerzo", 5000, w.betoM, w.anaM, w.betoM)
	w.collect()
	if got := w.after(30 * time.Second); len(got) != 0 {
		t.Fatalf("sent %+v within the minute", got)
	}
	if p := only(t, w.after(push.Spacing-30*time.Second), w.anaPhone); p.Items[0].Description != "Almuerzo" {
		t.Errorf("items = %+v", p.Items)
	}
}

func TestEachLedgerHasItsOwnNotifications(t *testing.T) {
	w := newWorld(t)
	other := w.trip
	w.trip = uuid.NewString()
	usd := "USD"
	w.write(func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(context.Background(), store.CreateLedgerParams{ID: w.trip, Kind: "group", Name: "Casa", Currency: &usd, Version: version, CreatedAt: time.Now()}); err != nil {
			return err
		}
		anaM, betoM := uuid.NewString(), uuid.NewString()
		if err := q.CreateMember(context.Background(), store.CreateMemberParams{ID: anaM, LedgerID: w.trip, UserID: &w.ana, Name: "Ana", Version: version, CreatedAt: time.Now()}); err != nil {
			return err
		}
		w.anaM, w.betoM = anaM, betoM
		return q.CreateMember(context.Background(), store.CreateMemberParams{ID: betoM, LedgerID: w.trip, UserID: &w.beto, Name: "Beto", Version: version, CreatedAt: time.Now()})
	})
	w.expense(w.beto, "Luz", 6000, w.betoM, w.anaM, w.betoM)
	w.collect()

	got := w.after(push.Wait)
	if len(got) != 1 || got[0].payload.Title != "Casa" || got[0].payload.LedgerID == other {
		t.Fatalf("sent %+v, want one about Casa", got)
	}
}

func TestARecordDeletedBeforeTheNotificationIsLeftOut(t *testing.T) {
	w := newWorld(t)
	id := w.expense(w.beto, "Error", 1000, w.betoM, w.anaM, w.betoM)
	w.collect()
	if err := sync.DeleteExpense(context.Background(), w.db, w.beto, id); err != nil {
		t.Fatal(err)
	}
	w.collect()
	if got := w.after(push.Wait); len(got) != 0 {
		t.Fatalf("sent %+v about a deleted Expense", got)
	}
}

func TestOnlyDevicesWithNotificationsOnInLiveSessions(t *testing.T) {
	w := newWorld(t)
	// Ana's second phone signed in long ago and never came back.
	old := w.subscribe(w.ana, time.Now().Add(-100*24*time.Hour))
	// Dani's account has no device with notifications on.
	dani := w.user("Dani")
	daniM := uuid.NewString()
	w.write(func(q *store.Queries, version int64) error {
		return q.CreateMember(context.Background(), store.CreateMemberParams{ID: daniM, LedgerID: w.trip, UserID: &dani, Name: "Dani", Version: version, CreatedAt: time.Now()})
	})
	w.expense(w.beto, "Hotel", 8000, w.betoM, w.anaM, w.betoM, daniM)
	w.collect()

	got := w.after(push.Wait)
	if len(got) != 1 || got[0].endpoint != w.anaPhone || got[0].endpoint == old {
		t.Fatalf("sent %+v, want only Ana's current phone", got)
	}
}

func TestAGoneSubscriptionIsDeleted(t *testing.T) {
	w := newWorld(t)
	w.sender.gone[w.anaPhone] = true
	w.expense(w.beto, "Cena", 1000, w.betoM, w.anaM, w.betoM)
	w.collect()
	w.after(push.Wait)

	devices, err := store.New(w.db).ListPushSubscriptionsOfUser(context.Background(), store.ListPushSubscriptionsOfUserParams{UserID: w.ana, UsedAfter: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 0 {
		t.Errorf("Ana still has %d subscriptions", len(devices))
	}
}

func TestTwoServersNotifyEachRecordOnce(t *testing.T) {
	w := newWorld(t)
	second := w.notifier()
	w.expense(w.beto, "Cena", 1000, w.betoM, w.anaM, w.betoM)
	w.collect()
	if err := second.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.now = w.now.Add(push.Wait)
	second.SendDue(context.Background())
	if got := w.sender.take(); len(got) != 0 {
		t.Fatalf("the second server sent %+v", got)
	}
	only(t, w.after(0), w.anaPhone)
}

func TestLateRecordsAreNotNotified(t *testing.T) {
	w := newWorld(t)
	w.expense(w.beto, "Cena", 1000, w.betoM, w.anaM, w.betoM)
	// Push was off, or the server was busy, for longer than Freshness.
	w.now = w.now.Add(push.Freshness + time.Minute)
	w.collect()
	if got := w.after(push.Wait); len(got) != 0 {
		t.Fatalf("sent %+v about an old Expense", got)
	}
}

func TestAFriendshipIsNamedAfterTheFriend(t *testing.T) {
	w := newWorld(t)
	w.trip = uuid.NewString()
	w.write(func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(context.Background(), store.CreateLedgerParams{ID: w.trip, Kind: "friendship", Version: version, CreatedAt: time.Now()}); err != nil {
			return err
		}
		w.anaM, w.betoM = uuid.NewString(), uuid.NewString()
		if err := q.CreateMember(context.Background(), store.CreateMemberParams{ID: w.anaM, LedgerID: w.trip, UserID: &w.ana, Name: "Ana", Version: version, CreatedAt: time.Now()}); err != nil {
			return err
		}
		return q.CreateMember(context.Background(), store.CreateMemberParams{ID: w.betoM, LedgerID: w.trip, UserID: &w.beto, Name: "Beto", Version: version, CreatedAt: time.Now()})
	})
	w.expense(w.beto, "Café", 800, w.betoM, w.anaM, w.betoM)
	w.collect()
	if p := only(t, w.after(push.Wait), w.anaPhone); p.Title != "Beto" {
		t.Errorf("title = %q, want Beto", p.Title)
	}
}

func TestALongBurstKeepsTheNewestRecords(t *testing.T) {
	w := newWorld(t)
	for i := range 10 {
		w.expense(w.beto, "Gasto "+string(rune('A'+i)), 100, w.betoM, w.anaM, w.betoM)
	}
	w.collect()
	p := only(t, w.after(push.Wait), w.anaPhone)
	if len(p.Items) != 8 || p.More != 2 || p.Items[0].Description != "Gasto C" || p.Items[7].Description != "Gasto J" {
		t.Errorf("payload = %+v", p)
	}
}

func TestRunCollectsOnEachChangeAndSendsWhatWaitsWhenItStops(t *testing.T) {
	w := newWorld(t)
	n := push.New(push.Config{DB: w.db, Sender: w.sender, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	feed := &fakeFeed{changes: make(chan changefeed.Change, 1)}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		n.Run(ctx, feed)
		close(done)
	}()

	w.expense(w.beto, "Cena", 1000, w.betoM, w.anaM, w.betoM)
	feed.changes <- changefeed.Change{LedgerID: w.trip}
	// Run reads the change before the wait is over; stopping sends it.
	time.Sleep(200 * time.Millisecond)
	stop()
	<-done
	only(t, w.sender.take(), w.anaPhone)
}

type fakeFeed struct{ changes chan changefeed.Change }

func (f *fakeFeed) Subscribe() (<-chan changefeed.Change, func()) { return f.changes, func() {} }
