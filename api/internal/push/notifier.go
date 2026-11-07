// Package push sends Web Push notifications of new Expenses and Settlements to
// the devices of the Users they involve (ADR 0013, docs/reglas-de-negocio.md).
//
// The Notifier reads the Activities after its cursor each time the change
// feed says a ledger changed. It queues one notification per User and ledger,
// waits a few seconds for more records of that ledger, and sends at most one
// per minute. What it sends is composed when it sends, from the records as
// they are then: a record deleted in the meantime is left out.
package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

const (
	// Wait is how long a notification waits for more records of its ledger,
	// so several saves in a row arrive as one notification.
	Wait = 5 * time.Second
	// Spacing is the least time between two notifications of one ledger to
	// one User.
	Spacing = time.Minute
	// Freshness is how old an Activity can be and still be notified. Older
	// ones, such as those written while push was off, are skipped.
	Freshness = 15 * time.Minute
	// maxItems is the most records a notification describes; it counts the
	// older ones in More.
	maxItems = 8
	// maxPayload keeps a notification under the 4 KB that every push service
	// takes, with room for the encryption.
	maxPayload = 3000
)

const (
	kindExpense    = "expense_created"
	kindSettlement = "settlement_created"
)

// Subscription is a device to notify: the endpoint of its push service and
// the keys that encrypt for it.
type Subscription struct {
	Endpoint, P256dh, Auth string
}

// Sender delivers one notification to a device.
type Sender interface {
	Send(ctx context.Context, to Subscription, payload []byte) error
}

// ErrGone is what a Sender returns when the push service says the
// subscription no longer exists. The Notifier then deletes it.
var ErrGone = errors.New("the push subscription no longer exists")

// Feed says when a ledger changed: *changefeed.Feed.
type Feed interface {
	Subscribe() (<-chan changefeed.Change, func())
}

// Payload is what the service worker receives, as JSON. It composes the text
// in the device's language and formats the amounts in its region.
type Payload struct {
	LedgerID string `json:"ledgerId"`
	// Title is the Group's name, or the friend's name in a Friendship.
	Title string `json:"title"`
	// Items are the records, oldest first.
	Items []Item `json:"items"`
	// More counts the older records left out to keep the notification short.
	More int `json:"more,omitempty"`
}

// Item is one Expense or Settlement in a notification.
type Item struct {
	// Kind is "expense" or "settlement".
	Kind string `json:"kind"`
	// Actor is the name of the Member who wrote it; empty if unknown.
	Actor    string `json:"actor"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	// Description and Share are of an Expense: Share is the recipient's
	// OwedShare, which may be zero.
	Description string `json:"description,omitempty"`
	Share       *int64 `json:"share,omitempty"`
	// You, Counterpart and ByCounterpart are of a Settlement. You is "paid"
	// when the recipient paid it and "received" when they got it;
	// Counterpart is the other Member's name, and ByCounterpart says that
	// Member wrote it.
	You           string `json:"you,omitempty"`
	Counterpart   string `json:"counterpart,omitempty"`
	ByCounterpart bool   `json:"byCounterpart,omitempty"`
}

// Notifier queues and sends the notifications. Run drives it; tests call
// Collect and SendDue with their own clock.
type Notifier struct {
	db     *sql.DB
	sender Sender
	logger *slog.Logger
	now    func() time.Time

	pending  map[key]*batch
	lastSent map[key]time.Time
}

// Config is what New needs. Now is nil for time.Now.
type Config struct {
	DB     *sql.DB
	Sender Sender
	Logger *slog.Logger
	Now    func() time.Time
}

func New(c Config) *Notifier {
	now := c.Now
	if now == nil {
		now = time.Now
	}
	return &Notifier{db: c.DB, sender: c.Sender, logger: c.Logger, now: now, pending: map[key]*batch{}, lastSent: map[key]time.Time{}}
}

// key is a notification in the making: one User, one ledger.
type key struct{ userID, ledgerID string }

// notice is a record to tell a User about.
type notice struct {
	kind     string
	actorID  string
	recordID string
}

type batch struct {
	notices []notice
	due     time.Time
}

// Run collects and sends until ctx ends, then sends what is still waiting,
// so a restart of the server loses nothing.
func (n *Notifier) Run(ctx context.Context, feed Feed) {
	changes, cancel := feed.Subscribe()
	defer cancel()
	n.collect(ctx)
	for {
		var due <-chan time.Time
		if next, ok := n.NextDue(); ok {
			due = time.After(next.Sub(n.now()))
		}
		select {
		case <-ctx.Done():
			flush, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			n.sendAll(flush)
			cancel()
			return
		case <-changes:
			// One read covers every change already waiting.
			for len(changes) > 0 {
				<-changes
			}
			n.collect(ctx)
		case <-due:
			n.SendDue(ctx)
		}
	}
}

// collect is Collect for Run: a failure is logged, and the next change tries
// again from the same cursor.
func (n *Notifier) collect(ctx context.Context) {
	if err := n.Collect(ctx); err != nil && ctx.Err() == nil {
		n.logger.Error("push: reading the new records failed", "err", err)
	}
}

// Collect reads the new Expenses and Settlements since the last call, in this
// instance or another one, and queues a notification for each User they
// involve who has a device with notifications on. It never queues one for
// the User who wrote the record.
func (n *Notifier) Collect(ctx context.Context) error {
	now := n.now()
	tx, err := n.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := store.New(tx)

	after, err := q.TakePushProgress(ctx)
	if err != nil {
		return fmt.Errorf("take the push progress: %w", err)
	}
	// Writes take their versions in commit order, so every Activity up to
	// the committed counter is already visible.
	upto, err := q.CurrentVersion(ctx)
	if err != nil {
		return err
	}
	if upto <= after {
		return nil
	}
	activities, err := q.ListNotifiableActivities(ctx, store.ListNotifiableActivitiesParams{After: after, Upto: upto, FreshAfter: now.Add(-Freshness)})
	if err != nil {
		return err
	}
	type queued struct {
		key    key
		notice notice
	}
	var found []queued
	for _, a := range activities {
		x, recipients, err := involved(ctx, q, a, now)
		if err != nil {
			return err
		}
		for _, userID := range recipients {
			found = append(found, queued{key{userID, a.LedgerID}, x})
		}
	}
	if err := q.SetPushProgress(ctx, upto); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, f := range found {
		n.queue(f.key, f.notice, now)
	}
	return nil
}

// involved is the record of activity a and the Users to notify of it: those
// of the active Members it involves, except its writer, with a device whose
// notifications are on.
func involved(ctx context.Context, q *store.Queries, a store.Activity, now time.Time) (notice, []string, error) {
	var details struct {
		ExpenseID    string `json:"expenseId"`
		SettlementID string `json:"settlementId"`
	}
	if err := json.Unmarshal([]byte(a.Details), &details); err != nil {
		return notice{}, nil, fmt.Errorf("details of activity %s: %w", a.ID, err)
	}
	x := notice{kind: a.Kind, actorID: a.UserID}
	var memberIDs []string
	switch a.Kind {
	case kindExpense:
		x.recordID = details.ExpenseID
		paid, err := q.ListPaidSharesOfExpense(ctx, x.recordID)
		if err != nil {
			return notice{}, nil, err
		}
		owed, err := q.ListOwedSharesOfExpense(ctx, x.recordID)
		if err != nil {
			return notice{}, nil, err
		}
		for _, s := range paid {
			memberIDs = append(memberIDs, s.MemberID)
		}
		for _, s := range owed {
			memberIDs = append(memberIDs, s.MemberID)
		}
	case kindSettlement:
		x.recordID = details.SettlementID
		s, err := q.GetSettlement(ctx, x.recordID)
		if errors.Is(err, sql.ErrNoRows) {
			return x, nil, nil
		}
		if err != nil {
			return notice{}, nil, err
		}
		memberIDs = []string{s.FromMemberID, s.ToMemberID}
	}

	members, err := q.ListMembers(ctx, a.LedgerID)
	if err != nil {
		return notice{}, nil, err
	}
	var recipients []string
	for _, m := range members {
		if !slices.Contains(memberIDs, m.ID) || m.Status != "active" || m.UserID == nil || *m.UserID == a.UserID {
			continue
		}
		on, err := q.UserHasPushSubscription(ctx, store.UserHasPushSubscriptionParams{UserID: *m.UserID, UsedAfter: now.Add(-auth.SessionLifetime)})
		if err != nil {
			return notice{}, nil, err
		}
		if on {
			recipients = append(recipients, *m.UserID)
		}
	}
	return x, recipients, nil
}

// queue adds x to the notification of k. A new notification waits Wait, and
// longer if the last one of k went out less than Spacing ago.
func (n *Notifier) queue(k key, x notice, now time.Time) {
	b := n.pending[k]
	if b == nil {
		due := now.Add(Wait)
		if last, ok := n.lastSent[k]; ok && last.Add(Spacing).After(due) {
			due = last.Add(Spacing)
		}
		b = &batch{due: due}
		n.pending[k] = b
	}
	b.notices = append(b.notices, x)
}

// NextDue is when the next queued notification is due, if one is queued.
func (n *Notifier) NextDue() (time.Time, bool) {
	var next time.Time
	for _, b := range n.pending {
		if next.IsZero() || b.due.Before(next) {
			next = b.due
		}
	}
	return next, !next.IsZero()
}

// SendDue sends the queued notifications whose wait is over.
func (n *Notifier) SendDue(ctx context.Context) {
	now := n.now()
	for _, k := range n.keys() {
		if b := n.pending[k]; !b.due.After(now) {
			delete(n.pending, k)
			n.send(ctx, k, b.notices, now)
		}
	}
	for k, last := range n.lastSent {
		if now.Sub(last) >= Spacing {
			delete(n.lastSent, k)
		}
	}
}

// sendAll sends every queued notification now.
func (n *Notifier) sendAll(ctx context.Context) {
	now := n.now()
	for _, k := range n.keys() {
		n.send(ctx, k, n.pending[k].notices, now)
		delete(n.pending, k)
	}
}

// keys are the queued notifications in a fixed order.
func (n *Notifier) keys() []key {
	keys := make([]key, 0, len(n.pending))
	for k := range n.pending {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b key) int {
		return strings.Compare(a.userID+"/"+a.ledgerID, b.userID+"/"+b.ledgerID)
	})
	return keys
}

// send composes the notification of k and sends it to each device of its
// User. The minute of Spacing starts even if a device failed, so a push
// service that fails is not tried more often.
func (n *Notifier) send(ctx context.Context, k key, notices []notice, now time.Time) {
	var payload *Payload
	var subscriptions []store.PushSubscription
	err := store.Read(ctx, n.db, func(q *store.Queries) error {
		var err error
		if payload, err = compose(ctx, q, k, notices); err != nil || payload == nil {
			return err
		}
		subscriptions, err = q.ListPushSubscriptionsOfUser(ctx, store.ListPushSubscriptionsOfUserParams{UserID: k.userID, UsedAfter: now.Add(-auth.SessionLifetime)})
		return err
	})
	if err != nil {
		n.logger.Error("push: composing a notification failed", "err", err)
		return
	}
	if payload == nil || len(subscriptions) == 0 {
		return
	}
	body, err := encode(payload)
	if err != nil {
		n.logger.Error("push: encoding a notification failed", "err", err)
		return
	}
	n.lastSent[k] = now
	for _, s := range subscriptions {
		err := n.sender.Send(ctx, Subscription{Endpoint: s.Endpoint, P256dh: s.P256dh, Auth: s.Auth}, body)
		switch {
		case err == nil:
		case errors.Is(err, ErrGone):
			if err := store.New(n.db).DeletePushSubscription(ctx, s.Endpoint); err != nil {
				n.logger.Error("push: deleting a subscription failed", "err", err)
			}
		default:
			// The endpoint is a secret of the device: only its host goes to
			// the log.
			n.logger.Warn("push: sending a notification failed", "service", hostOf(s.Endpoint), "err", err)
		}
	}
}

// compose builds the notification of k from the records as they are now. It
// returns nil when nothing is left to tell: the records were deleted, they no
// longer involve the User, or the User left the ledger.
func compose(ctx context.Context, q *store.Queries, k key, notices []notice) (*Payload, error) {
	ledger, err := q.GetLedger(ctx, k.ledgerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ledger.DeletedAt != nil {
		return nil, nil
	}
	members, err := q.ListMembers(ctx, k.ledgerID)
	if err != nil {
		return nil, err
	}
	var me *store.Member
	byID := map[string]store.Member{}
	for i, m := range members {
		byID[m.ID] = m
		if m.UserID != nil && *m.UserID == k.userID && m.Status == "active" {
			me = &members[i]
		}
	}
	if me == nil {
		return nil, nil
	}
	nameOf := func(userID string) string {
		for _, m := range members {
			if m.UserID != nil && *m.UserID == userID {
				return m.Name
			}
		}
		return ""
	}

	p := &Payload{LedgerID: k.ledgerID, Title: ledger.Name}
	if ledger.Kind == "friendship" {
		for _, m := range members {
			if m.ID != me.ID {
				p.Title = m.Name
			}
		}
	}
	for _, x := range notices {
		item, ok, err := itemOf(ctx, q, x, me.ID, byID)
		if err != nil {
			return nil, err
		}
		if ok {
			item.Actor = nameOf(x.actorID)
			p.Items = append(p.Items, item)
		}
	}
	if len(p.Items) == 0 {
		return nil, nil
	}
	return p, nil
}

// itemOf describes the record of x to the Member me, if it still involves them.
func itemOf(ctx context.Context, q *store.Queries, x notice, me string, members map[string]store.Member) (Item, bool, error) {
	switch x.kind {
	case kindExpense:
		e, err := q.GetExpense(ctx, x.recordID)
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, false, nil
		}
		if err != nil || e.DeletedAt != nil {
			return Item{}, false, err
		}
		paid, err := q.ListPaidSharesOfExpense(ctx, e.ID)
		if err != nil {
			return Item{}, false, err
		}
		owed, err := q.ListOwedSharesOfExpense(ctx, e.ID)
		if err != nil {
			return Item{}, false, err
		}
		in := false
		var share int64
		for _, s := range paid {
			in = in || s.MemberID == me
		}
		for _, s := range owed {
			if s.MemberID == me {
				in, share = true, s.Amount
			}
		}
		if !in {
			return Item{}, false, nil
		}
		return Item{Kind: "expense", Description: e.Description, Amount: e.Total, Currency: e.Currency, Share: &share}, true, nil

	case kindSettlement:
		s, err := q.GetSettlement(ctx, x.recordID)
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, false, nil
		}
		if err != nil || s.DeletedAt != nil {
			return Item{}, false, err
		}
		item := Item{Kind: "settlement", Amount: s.Amount, Currency: s.Currency}
		var other store.Member
		switch me {
		case s.FromMemberID:
			item.You, other = "paid", members[s.ToMemberID]
		case s.ToMemberID:
			item.You, other = "received", members[s.FromMemberID]
		default:
			return Item{}, false, nil
		}
		item.Counterpart = other.Name
		item.ByCounterpart = other.UserID != nil && *other.UserID == x.actorID
		return item, true, nil
	}
	return Item{}, false, nil
}

// encode writes p as JSON, leaving out its oldest records until it fits.
func encode(p *Payload) ([]byte, error) {
	if extra := len(p.Items) - maxItems; extra > 0 {
		p.Items, p.More = p.Items[extra:], p.More+extra
	}
	for {
		body, err := json.Marshal(p)
		if err != nil || len(body) <= maxPayload || len(p.Items) == 1 {
			return body, err
		}
		p.Items, p.More = p.Items[1:], p.More+1
	}
}

func hostOf(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Host
}
