package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

// nameless signs in someone who came with a login code and has not said
// their name yet.
func (w *world) nameless(email string) *person {
	w.t.Helper()
	p := w.signIn(email)
	if _, err := w.db.Exec(`UPDATE users SET name = '' WHERE id = ?`, p.id); err != nil {
		w.t.Fatal(err)
	}
	p.name = ""
	return p
}

func TestWithoutANameThereAreNoNewGroups(t *testing.T) {
	w := newWorld(t)
	nadie := w.nameless("nadie")

	rec := nadie.send(http.MethodPost, "/api/v1/groups", `{"name":"Casa","currency":"COP"}`)

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeNameRequired)
	if changes := nadie.pull(); len(changes.Ledgers) != 0 {
		t.Errorf("ledgers = %+v, want none", changes.Ledgers)
	}

	nadie.send(http.MethodPatch, "/api/v1/me", `{"name":"Nadia"}`)
	groupID := nadie.createGroup(`{"name":"Casa","currency":"COP"}`)

	if members := membersOf(nadie.pull(), groupID); len(members) != 1 || members[0].Name != "Nadia" {
		t.Errorf("members = %+v, want only Nadia", members)
	}
}

func TestWithoutANameThereAreNoNewFriendships(t *testing.T) {
	w := newWorld(t)
	nadie := w.nameless("nadie")

	rec := nadie.send(http.MethodPost, "/api/v1/friendships", `{"name":"Beto"}`)

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeNameRequired)
	if changes := nadie.pull(); len(changes.Ledgers) != 0 {
		t.Errorf("ledgers = %+v, want none", changes.Ledgers)
	}
}

func TestWithoutANameNobodyJoinsAGroupAsANewMember(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	nadie := w.nameless("nadie")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	link := ana.groupLink(groupID)

	rec := nadie.send(http.MethodPost, "/api/v1/invites/"+link+"/join", "")

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeNameRequired)
	if members := membersOf(ana.pull(), groupID); len(members) != 1 {
		t.Errorf("members = %+v, want only Ana", members)
	}
}

// Claiming and coming back use a Member that already has a name.
func TestWithoutANameSomeoneCanClaimAPendingMemberAndComeBackToIt(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	nadie := w.nameless("nadie")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	link := ana.groupLink(groupID)
	betoID := ana.memberID(groupID, "Beto")

	decode[httpapi.JoinedLedger](t, nadie.claim(link, betoID), http.StatusOK)
	nadie.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")
	joined := nadie.join(link)

	if m := nadie.member(groupID, joined.MemberId.String()); m.ID != betoID || m.Name != "Beto" || m.Status != "active" {
		t.Errorf("member = %+v, want the active Member Beto", m)
	}
}

// Someone who only claimed a Member has no name of their own, so a Friend
// keeps the name the others already know them by.
func TestAFriendWithoutANameIsCalledWhatTheGroupCallsThem(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	nadie := w.nameless("nadie")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	decode[httpapi.JoinedLedger](t, nadie.claim(ana.groupLink(groupID), ana.memberID(groupID, "Beto")), http.StatusOK)

	friendshipID := ana.createFriendship(`{"userId":"` + nadie.id + `"}`)

	members := membersOf(ana.pull(), friendshipID)
	if len(members) != 2 {
		t.Fatalf("members = %+v, want Ana and her Friend", members)
	}
	for _, m := range members {
		if *m.UserID == nadie.id && m.Name != "Beto" {
			t.Errorf("Friend = %+v, want the name Beto", m)
		}
	}
}
