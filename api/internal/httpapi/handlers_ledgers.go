package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Groups, Members, Invite links and Friendships. Every change runs inside
// store.Write and leaves one Activity.

// failure is a request the rules refuse. Returned from inside store.Write it
// rolls the change back; the handler then answers it as a problem.
type failure struct {
	status int
	code   ProblemCode
	title  string
	detail string
}

func (f *failure) Error() string { return f.title }

func (f *failure) problem() Problem {
	p := Problem{Status: f.status, Code: f.code, Title: f.title}
	if f.detail != "" {
		p.Detail = &f.detail
	}
	return p
}

var errUnauthenticated = &failure{status: http.StatusUnauthorized, code: ProblemCodeUnauthenticated, title: "Sign in first."}

// invalid is a request that breaks a rule of the contract the generated code
// does not check, such as a length or a pattern.
func invalid(format string, args ...any) *failure {
	return &failure{
		status: http.StatusBadRequest,
		code:   ProblemCodeValidationFailed,
		title:  "The request does not match the API contract.",
		detail: fmt.Sprintf(format, args...),
	}
}

const (
	maxNameLength  = 80
	maxEmailLength = 254
	maxNewMembers  = 50
)

// cleanName trims a Group's or a Member's name and checks its length.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > maxNameLength {
		return "", invalid("a name must have between 1 and %d characters", maxNameLength)
	}
	return name, nil
}

// checkCurrency accepts the ISO 4217 codes the domain knows.
func checkCurrency(c Currency) error {
	if _, ok := domain.Currency(c).Decimals(); !ok || strings.ToUpper(c) != c {
		return invalid("%q is not an ISO 4217 currency code", c)
	}
	return nil
}

// pendingMember is a pending Member about to be stored.
type pendingMember struct {
	name  string
	email *string
}

func cleanPendingMember(m NewPendingMember) (pendingMember, error) {
	name, err := cleanName(m.Name)
	if err != nil {
		return pendingMember{}, err
	}
	p := pendingMember{name: name}
	if m.Email != nil {
		email := string(*m.Email)
		if len(email) > maxEmailLength {
			return pendingMember{}, invalid("an email must have at most %d characters", maxEmailLength)
		}
		p.email = &email
	}
	return p, nil
}

// answer turns err into the operation's problem response when it is a
// failure; any other error goes back to the strict handler, which logs it
// and answers 500.
func answer[R any](err error, respond func(status int, p Problem) R) (R, error) {
	var f *failure
	if errors.As(err, &f) {
		return respond(f.status, f.problem()), nil
	}
	var zero R
	return zero, err
}

// currentUser is the signed-in User, or errUnauthenticated.
func currentUser(ctx context.Context) (store.User, error) {
	user, ok := auth.CurrentUser(ctx)
	if !ok {
		return store.User{}, errUnauthenticated
	}
	return user, nil
}

// newID returns a UUIDv7 for a ledger, Member or Activity.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// activity records who changed what in a ledger; details depend on kind.
func activity(ctx context.Context, q *store.Queries, ledgerID, userID, kind string, details map[string]any, version int64) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("activity %s: %w", kind, err)
	}
	err = q.CreateActivity(ctx, store.CreateActivityParams{
		ID:        newID(),
		LedgerID:  ledgerID,
		UserID:    userID,
		Kind:      kind,
		Details:   string(raw),
		Version:   version,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("activity %s: %w", kind, err)
	}
	return nil
}

func groupResponse(l store.Ledger) Group {
	g := Group{
		Id:            uuid.MustParse(l.ID),
		Name:          l.Name,
		SimplifyDebts: l.SimplifyDebts,
		CreatedAt:     l.CreatedAt,
		Version:       l.Version,
	}
	if l.Currency != nil {
		g.Currency = *l.Currency
	}
	return g
}

// --- groups ---

func (s *Server) CreateGroup(ctx context.Context, req CreateGroupRequestObject) (CreateGroupResponseObject, error) {
	respond := func(status int, p Problem) CreateGroupResponseObject {
		return CreateGroupdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	body := req.Body
	name, err := cleanName(body.Name)
	if err != nil {
		return answer(err, respond)
	}
	if err := checkCurrency(body.Currency); err != nil {
		return answer(err, respond)
	}
	var pending []pendingMember
	if body.PendingMembers != nil {
		if len(*body.PendingMembers) > maxNewMembers {
			return answer(invalid("a Group starts with at most %d pending Members", maxNewMembers), respond)
		}
		for _, m := range *body.PendingMembers {
			p, err := cleanPendingMember(m)
			if err != nil {
				return answer(err, respond)
			}
			pending = append(pending, p)
		}
	}

	now := time.Now().UTC()
	ledger := store.Ledger{
		ID:            newID(),
		Kind:          "group",
		Name:          name,
		Currency:      &body.Currency,
		SimplifyDebts: body.SimplifyDebts == nil || *body.SimplifyDebts,
		CreatedAt:     now,
	}
	ledger.Version, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		err := q.CreateLedger(ctx, store.CreateLedgerParams{
			ID:            ledger.ID,
			Kind:          ledger.Kind,
			Name:          ledger.Name,
			Currency:      ledger.Currency,
			SimplifyDebts: ledger.SimplifyDebts,
			Version:       version,
			CreatedAt:     now,
		})
		if err != nil {
			return fmt.Errorf("create group: %w", err)
		}
		err = q.CreateMember(ctx, store.CreateMemberParams{
			ID:        newID(),
			LedgerID:  ledger.ID,
			UserID:    &user.ID,
			Name:      user.Name,
			Version:   version,
			CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("create the creator's member: %w", err)
		}
		for _, p := range pending {
			err := q.CreateMember(ctx, store.CreateMemberParams{
				ID:        newID(),
				LedgerID:  ledger.ID,
				Name:      p.name,
				Email:     p.email,
				Version:   version,
				CreatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("create pending member: %w", err)
			}
		}
		return activity(ctx, q, ledger.ID, user.ID, "group_created", map[string]any{"name": ledger.Name, "currency": body.Currency}, version)
	})
	if err != nil {
		return answer(err, respond)
	}
	return CreateGroup201JSONResponse(groupResponse(ledger)), nil
}
