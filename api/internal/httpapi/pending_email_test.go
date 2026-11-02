package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

// emailFor is the email of memberID that p's replica gets, or "" without one.
func (p *person) emailFor(memberID string) string {
	p.w.t.Helper()
	if m := p.syncedMember(memberID); m.Email != nil {
		return *m.Email
	}
	return ""
}

// Whoever gets the Invite link joins and would otherwise collect the emails of
// people who never used the app (finding 11 of docs/revision-seguridad.md).
func TestOnlyWhoAddedAPendingMembersEmailSeesIt(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Pepa","email":"pepa@test.example"}]}`)
	beto.join(ana.groupLink(groupID))
	pepaID := ana.memberID(groupID, "Pepa")

	if got := ana.emailFor(pepaID); got != "pepa@test.example" {
		t.Errorf("Ana, who added Pepa, gets email %q", got)
	}
	if got := beto.emailFor(pepaID); got != "" {
		t.Errorf("Beto, who joined through the link, gets email %q, want none", got)
	}
}

// editPending makes p replace the name and email of the pending memberID.
func (p *person) editPending(groupID, memberID, body string) httpapi.Member {
	p.w.t.Helper()
	return decode[httpapi.Member](p.w.t, p.send(http.MethodPut, "/api/v1/groups/"+groupID+"/members/"+memberID, body), http.StatusOK)
}

// Beto never saw the email, so he cannot be expected to send it back.
func TestRenamingAPendingMemberKeepsAnEmailTheEditorCannotSee(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Pepa","email":"pepa@test.example"}]}`)
	beto.join(ana.groupLink(groupID))
	pepaID := ana.memberID(groupID, "Pepa")

	edited := beto.editPending(groupID, pepaID, `{"name":"Pepa Ríos"}`)

	if edited.Name != "Pepa Ríos" || edited.Email != nil {
		t.Errorf("Beto's answer = %+v, want the new name and no email", edited)
	}
	if got := ana.emailFor(pepaID); got != "pepa@test.example" {
		t.Errorf("Ana gets email %q, want hers kept", got)
	}
	if got := beto.emailFor(pepaID); got != "" {
		t.Errorf("Beto gets email %q, want none", got)
	}
}

// Whoever writes an email is who sees it from then on, and who can remove it.
func TestAPendingMembersEmailIsWhoeverWroteItLast(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Pepa","email":"pepa@test.example"}]}`)
	beto.join(ana.groupLink(groupID))
	pepaID := ana.memberID(groupID, "Pepa")

	edited := beto.editPending(groupID, pepaID, `{"name":"Pepa","email":"pepa@otro.example"}`)

	if edited.Email == nil || *edited.Email != "pepa@otro.example" {
		t.Errorf("Beto's answer = %+v, want the email he wrote", edited)
	}
	if got := ana.emailFor(pepaID); got != "" {
		t.Errorf("Ana gets email %q, want none: Beto replaced hers", got)
	}
	if got := beto.emailFor(pepaID); got != "pepa@otro.example" {
		t.Errorf("Beto gets email %q", got)
	}

	beto.editPending(groupID, pepaID, `{"name":"Pepa"}`)

	if m := ana.member(groupID, pepaID); m.Email != nil {
		t.Errorf("Pepa = %+v, want no email once Beto left his out", m)
	}
}

func TestWhoAddsAPendingFriendStillSeesTheirEmail(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	friendshipID := ana.createFriendship(`{"name":"Pepa","email":"pepa@test.example"}`)

	if got := ana.emailFor(ana.memberID(friendshipID, "Pepa")); got != "pepa@test.example" {
		t.Errorf("Ana gets email %q for her pending Friend", got)
	}
}
