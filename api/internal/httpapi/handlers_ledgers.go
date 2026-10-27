package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/invites"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
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

// errNotFound answers for a ledger the User cannot see, the same as for one
// that does not exist.
var errNotFound = &failure{status: http.StatusNotFound, code: ProblemCodeNotFound, title: "Not found."}

// open loads the ledger ledgerID, which must be of kind and alive, with the
// caller's active Member in it. Any other case is errNotFound: every query is
// limited to the ledgers where the User has an active Member.
func open(ctx context.Context, q *store.Queries, ledgerID, kind, userID string) (ledgers.Snapshot, store.Member, error) {
	snap, err := ledgers.Load(ctx, q, ledgerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ledgers.Snapshot{}, store.Member{}, errNotFound
	}
	if err != nil {
		return ledgers.Snapshot{}, store.Member{}, err
	}
	if snap.Ledger.Kind != kind || snap.Ledger.DeletedAt != nil {
		return ledgers.Snapshot{}, store.Member{}, errNotFound
	}
	for _, m := range snap.Members {
		if m.UserID != nil && *m.UserID == userID && m.Status == "active" {
			return snap, m, nil
		}
	}
	return ledgers.Snapshot{}, store.Member{}, errNotFound
}

var errNonzeroBalance = &failure{status: http.StatusConflict, code: ProblemCodeNonzeroBalance, title: "Some Balance is not zero."}

// read runs fn in a read-only transaction, so everything it loads comes from
// the same snapshot.
func (s *Server) read(ctx context.Context, fn func(q *store.Queries) error) error {
	tx, err := s.deps.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin read: %w", err)
	}
	defer tx.Rollback()
	return fn(store.New(tx))
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

func (s *Server) UpdateGroup(ctx context.Context, req UpdateGroupRequestObject) (UpdateGroupResponseObject, error) {
	respond := func(status int, p Problem) UpdateGroupResponseObject {
		return UpdateGroupdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	body := req.Body
	if body.Name == nil && body.SimplifyDebts == nil && body.Currency == nil {
		return answer(invalid("send at least one of name, simplifyDebts and currency"), respond)
	}
	var name string
	if body.Name != nil {
		if name, err = cleanName(*body.Name); err != nil {
			return answer(err, respond)
		}
	}
	if body.Currency != nil {
		if err := checkCurrency(*body.Currency); err != nil {
			return answer(err, respond)
		}
	}

	var group store.Ledger
	_, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		snap, _, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		group = snap.Ledger
		renamed := body.Name != nil && name != group.Name
		toggled := body.SimplifyDebts != nil && *body.SimplifyDebts != group.SimplifyDebts
		relabeled := body.Currency != nil && *body.Currency != *group.Currency

		if relabeled {
			if err := relabel(ctx, q, snap, domain.Currency(*body.Currency), version); err != nil {
				return err
			}
			if err := activity(ctx, q, group.ID, user.ID, "group_currency_changed", map[string]any{"from": *group.Currency, "to": *body.Currency}, version); err != nil {
				return err
			}
		}
		if renamed || toggled {
			params := store.UpdateLedgerParams{ID: group.ID, Name: group.Name, SimplifyDebts: group.SimplifyDebts, Version: version}
			if renamed {
				params.Name = name
			}
			if toggled {
				params.SimplifyDebts = *body.SimplifyDebts
			}
			if _, err := q.UpdateLedger(ctx, params); err != nil {
				return fmt.Errorf("update group: %w", err)
			}
		}
		if renamed {
			if err := activity(ctx, q, group.ID, user.ID, "group_renamed", map[string]any{"from": group.Name, "to": name}, version); err != nil {
				return err
			}
		}
		if toggled {
			if err := activity(ctx, q, group.ID, user.ID, "debt_simplification_changed", map[string]any{"simplifyDebts": *body.SimplifyDebts}, version); err != nil {
				return err
			}
		}
		if !relabeled && !renamed && !toggled {
			return nil
		}
		if group, err = q.GetLedger(ctx, group.ID); err != nil {
			return fmt.Errorf("get updated group: %w", err)
		}
		return nil
	})
	var lost *domain.RelabelError
	if errors.As(err, &lost) {
		return UpdateGroup409ApplicationProblemPlusJSONResponse(precisionLoss(lost)), nil
	}
	if err != nil {
		return answer(err, respond)
	}
	return UpdateGroup200JSONResponse(groupResponse(group)), nil
}

// relabel changes the currency of the Group in snap to to, with every
// Expense and Settlement, deleted ones included because they can be
// restored. It keeps the numbers people read and never converts; if some
// amount would lose decimals it returns the *domain.RelabelError.
func relabel(ctx context.Context, q *store.Queries, snap ledgers.Snapshot, to domain.Currency, version int64) error {
	if _, _, err := domain.Relabel(snap.Expenses(), snap.Settlements(), to); err != nil {
		return err
	}
	from := domain.Currency(*snap.Ledger.Currency)
	fromDecimals, ok := from.Decimals()
	if !ok {
		return fmt.Errorf("group %s has the unknown currency %q", snap.Ledger.ID, from)
	}
	toDecimals, _ := to.Decimals()
	if err := q.RelabelCurrency(ctx, snap.Ledger.ID, string(to), version); err != nil {
		return fmt.Errorf("relabel group: %w", err)
	}
	// RelabelCurrency only changes the labels; the minor units change with
	// the decimals (10.00 USD is 1000 cents, 10 JPY is 10).
	if err := q.ScaleLedgerAmounts(ctx, snap.Ledger.ID, fromDecimals, toDecimals); err != nil {
		return fmt.Errorf("relabel group: %w", err)
	}
	return nil
}

// precisionLoss answers a currency change that would drop decimals, with the
// records to fix first.
func precisionLoss(lost *domain.RelabelError) CurrencyPrecisionProblem {
	p := CurrencyPrecisionProblem{
		Status:        http.StatusConflict,
		Code:          ProblemCodeCurrencyPrecisionLoss,
		Title:         "Some amounts would lose decimals in the new currency.",
		ExpenseIds:    make([]uuid.UUID, len(lost.ExpenseIDs)),
		SettlementIds: make([]uuid.UUID, len(lost.SettlementIDs)),
	}
	for i, id := range lost.ExpenseIDs {
		p.ExpenseIds[i] = uuid.MustParse(id)
	}
	for i, id := range lost.SettlementIDs {
		p.SettlementIds[i] = uuid.MustParse(id)
	}
	return p
}

func (s *Server) DeleteGroup(ctx context.Context, req DeleteGroupRequestObject) (DeleteGroupResponseObject, error) {
	respond := func(status int, p Problem) DeleteGroupResponseObject {
		return DeleteGroupdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	_, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		snap, _, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		// Balances leaves out the zero ones.
		if len(snap.Balances()) > 0 {
			return errNonzeroBalance
		}
		now := time.Now().UTC()
		if _, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: snap.Ledger.ID, DeletedAt: &now, Version: version}); err != nil {
			return fmt.Errorf("delete group: %w", err)
		}
		return nil
	})
	if err != nil {
		return answer(err, respond)
	}
	return DeleteGroup204Response{}, nil
}

// --- members ---

func memberResponse(m store.Member) Member {
	r := Member{
		Id:       uuid.MustParse(m.ID),
		LedgerId: uuid.MustParse(m.LedgerID),
		Name:     m.Name,
		Status:   MemberStatus(m.Status),
		Version:  m.Version,
	}
	if m.UserID != nil {
		id := uuid.MustParse(*m.UserID)
		r.UserId = &id
	}
	if m.Email != nil {
		email := openapi_types.Email(*m.Email)
		r.Email = &email
	}
	return r
}

func (s *Server) AddPendingMember(ctx context.Context, req AddPendingMemberRequestObject) (AddPendingMemberResponseObject, error) {
	respond := func(status int, p Problem) AddPendingMemberResponseObject {
		return AddPendingMemberdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	pending, err := cleanPendingMember(*req.Body)
	if err != nil {
		return answer(err, respond)
	}
	var member store.Member
	_, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		snap, _, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		member = store.Member{
			ID:        newID(),
			LedgerID:  snap.Ledger.ID,
			Name:      pending.name,
			Email:     pending.email,
			Status:    "active",
			Version:   version,
			CreatedAt: time.Now().UTC(),
		}
		err = q.CreateMember(ctx, store.CreateMemberParams{
			ID:        member.ID,
			LedgerID:  member.LedgerID,
			Name:      member.Name,
			Email:     member.Email,
			Version:   version,
			CreatedAt: member.CreatedAt,
		})
		if err != nil {
			return fmt.Errorf("add pending member: %w", err)
		}
		return activity(ctx, q, member.LedgerID, user.ID, "member_added", map[string]any{"memberId": member.ID, "name": member.Name}, version)
	})
	if err != nil {
		return answer(err, respond)
	}
	return AddPendingMember201JSONResponse(memberResponse(member)), nil
}

// memberOf finds the Member memberID in snap, in any status.
func memberOf(snap ledgers.Snapshot, memberID string) (store.Member, bool) {
	for _, m := range snap.Members {
		if m.ID == memberID {
			return m, true
		}
	}
	return store.Member{}, false
}

// retire turns the active Member m into a former member, who stays on past
// Expenses but no longer sees the Group. Only with Balance zero: nobody leaves
// or is removed owing or being owed.
func retire(ctx context.Context, q *store.Queries, snap ledgers.Snapshot, m store.Member, version int64) error {
	for _, b := range snap.Balances() {
		if b.MemberID == m.ID {
			return errNonzeroBalance
		}
	}
	if _, err := q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: m.ID, Version: version}); err != nil {
		return fmt.Errorf("mark member former: %w", err)
	}
	return nil
}

func (s *Server) RemoveMember(ctx context.Context, req RemoveMemberRequestObject) (RemoveMemberResponseObject, error) {
	respond := func(status int, p Problem) RemoveMemberResponseObject {
		return RemoveMemberdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	_, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		snap, _, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		m, ok := memberOf(snap, req.MemberId.String())
		if !ok || m.Status != "active" {
			return errNotFound
		}
		if err := retire(ctx, q, snap, m, version); err != nil {
			return err
		}
		return activity(ctx, q, snap.Ledger.ID, user.ID, "member_removed", map[string]any{"memberId": m.ID}, version)
	})
	if err != nil {
		return answer(err, respond)
	}
	return RemoveMember204Response{}, nil
}

func (s *Server) LeaveGroup(ctx context.Context, req LeaveGroupRequestObject) (LeaveGroupResponseObject, error) {
	respond := func(status int, p Problem) LeaveGroupResponseObject {
		return LeaveGroupdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	_, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		snap, me, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		if err := retire(ctx, q, snap, me, version); err != nil {
			return err
		}
		return activity(ctx, q, snap.Ledger.ID, user.ID, "member_left", map[string]any{"memberId": me.ID}, version)
	})
	if err != nil {
		return answer(err, respond)
	}
	return LeaveGroup204Response{}, nil
}

// --- invite links ---

var errInviteLinkInvalid = &failure{status: http.StatusNotFound, code: ProblemCodeInviteLinkInvalid, title: "The Invite link does not exist or was revoked."}

// inviteLink is the current Invite link of l (ADR 0010).
func (s *Server) inviteLink(l store.Ledger) (InviteLink, error) {
	token, err := invites.Token(s.deps.InviteSecret, l.ID, l.InviteLinkGeneration)
	if err != nil {
		return InviteLink{}, err
	}
	return InviteLink{Token: token}, nil
}

// openInvite loads the live ledger whose current Invite link is token.
func (s *Server) openInvite(ctx context.Context, q *store.Queries, token string) (ledgers.Snapshot, error) {
	ledgerID, ok := invites.LedgerID(token)
	if !ok {
		return ledgers.Snapshot{}, errInviteLinkInvalid
	}
	snap, err := ledgers.Load(ctx, q, ledgerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ledgers.Snapshot{}, errInviteLinkInvalid
	}
	if err != nil {
		return ledgers.Snapshot{}, err
	}
	if snap.Ledger.DeletedAt != nil || !invites.Valid(s.deps.InviteSecret, token, snap.Ledger.InviteLinkGeneration) {
		return ledgers.Snapshot{}, errInviteLinkInvalid
	}
	return snap, nil
}

// claimable are the pending Members of snap that can still be claimed.
func claimable(snap ledgers.Snapshot) []store.Member {
	var out []store.Member
	for _, m := range snap.Members {
		if m.UserID == nil && m.Status == "active" {
			out = append(out, m)
		}
	}
	return out
}

// heldBy is userID's Member in snap, in any status.
func heldBy(snap ledgers.Snapshot, userID string) (store.Member, bool) {
	for _, m := range snap.Members {
		if m.UserID != nil && *m.UserID == userID {
			return m, true
		}
	}
	return store.Member{}, false
}

func (s *Server) GetGroupInviteLink(ctx context.Context, req GetGroupInviteLinkRequestObject) (GetGroupInviteLinkResponseObject, error) {
	respond := func(status int, p Problem) GetGroupInviteLinkResponseObject {
		return GetGroupInviteLinkdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	var link InviteLink
	err = s.read(ctx, func(q *store.Queries) error {
		snap, _, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		link, err = s.inviteLink(snap.Ledger)
		return err
	})
	if err != nil {
		return answer(err, respond)
	}
	return GetGroupInviteLink200JSONResponse(link), nil
}

func (s *Server) RegenerateGroupInviteLink(ctx context.Context, req RegenerateGroupInviteLinkRequestObject) (RegenerateGroupInviteLinkResponseObject, error) {
	respond := func(status int, p Problem) RegenerateGroupInviteLinkResponseObject {
		return RegenerateGroupInviteLinkdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	user, err := currentUser(ctx)
	if err != nil {
		return answer(err, respond)
	}
	var link InviteLink
	_, err = store.Write(ctx, s.deps.DB, func(q *store.Queries, version int64) error {
		snap, _, err := open(ctx, q, req.GroupId.String(), "group", user.ID)
		if err != nil {
			return err
		}
		link, err = s.regenerate(ctx, q, snap.Ledger.ID, user.ID, version)
		return err
	})
	if err != nil {
		return answer(err, respond)
	}
	return RegenerateGroupInviteLink200JSONResponse(link), nil
}

// regenerate moves the ledger's Invite link to its next generation, which
// revokes the previous token, and returns the new link.
func (s *Server) regenerate(ctx context.Context, q *store.Queries, ledgerID, userID string, version int64) (InviteLink, error) {
	ledger, err := q.RegenerateInviteLink(ctx, store.RegenerateInviteLinkParams{ID: ledgerID, Version: version})
	if err != nil {
		return InviteLink{}, fmt.Errorf("regenerate invite link: %w", err)
	}
	if err := activity(ctx, q, ledgerID, userID, "invite_link_regenerated", nil, version); err != nil {
		return InviteLink{}, err
	}
	return s.inviteLink(ledger)
}

func (s *Server) GetInvite(ctx context.Context, req GetInviteRequestObject) (GetInviteResponseObject, error) {
	respond := func(status int, p Problem) GetInviteResponseObject {
		return GetInvitedefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: status}
	}
	// The invite page shows the Group before signing in, so the session is optional.
	user, signedIn := auth.CurrentUser(ctx)
	var invite Invite
	err := s.read(ctx, func(q *store.Queries) error {
		snap, err := s.openInvite(ctx, q, req.Token)
		if err != nil {
			return err
		}
		invite = Invite{
			Kind:           InviteKind(snap.Ledger.Kind),
			LedgerId:       uuid.MustParse(snap.Ledger.ID),
			GroupName:      &snap.Ledger.Name,
			PendingMembers: []InviteMember{},
		}
		for _, m := range claimable(snap) {
			invite.PendingMembers = append(invite.PendingMembers, InviteMember{Id: uuid.MustParse(m.ID), Name: m.Name})
			// Only a suggestion: claiming is always the User's choice.
			if signedIn && invite.SuggestedMemberId == nil && m.Email != nil && strings.EqualFold(*m.Email, user.Email) {
				id := uuid.MustParse(m.ID)
				invite.SuggestedMemberId = &id
			}
		}
		if m, ok := heldBy(snap, user.ID); signedIn && ok && m.Status == "active" {
			id := uuid.MustParse(m.ID)
			invite.JoinedMemberId = &id
		}
		return nil
	})
	if err != nil {
		return answer(err, respond)
	}
	return GetInvite200JSONResponse(invite), nil
}
