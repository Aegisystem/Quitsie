package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

// removeFromGroup makes p remove memberID from groupID.
func (p *person) removeFromGroup(groupID, memberID string) {
	p.w.t.Helper()
	if rec := p.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+memberID, ""); rec.Code != http.StatusNoContent {
		p.w.t.Fatalf("remove: code = %d (body %s)", rec.Code, rec.Body)
	}
}

// Removing someone would mean nothing if the link they keep in their chat let
// them straight back in (finding 8 of docs/revision-seguridad.md).
func TestARemovedUserCannotComeBackThroughTheLink(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	link := ana.groupLink(groupID)
	joined := beto.join(link)
	ana.removeFromGroup(groupID, joined.MemberId.String())

	rec := beto.send(http.MethodPost, "/api/v1/invites/"+link+"/join", "")

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeRemovedFromGroup)
	if m := ana.member(groupID, joined.MemberId.String()); m.Status != "former" {
		t.Errorf("Beto's member = %+v, want still former", m)
	}
}

// readmit makes p readmit memberID to groupID.
func (p *person) readmit(groupID, memberID string) *httptest.ResponseRecorder {
	p.w.t.Helper()
	return p.send(http.MethodPost, "/api/v1/groups/"+groupID+"/members/"+memberID+"/readmit", "")
}

func TestAReadmittedUserComesBackThroughTheLinkWithTheirHistory(t *testing.T) {
	w := newWorld(t)
	ana, beto, carla := w.signIn("Ana"), w.signIn("Beto"), w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	link := ana.groupLink(groupID)
	joined := beto.join(link)
	carla.join(link)
	ana.removeFromGroup(groupID, joined.MemberId.String())

	rec := carla.readmit(groupID, joined.MemberId.String())

	if rec.Code != http.StatusNoContent {
		t.Fatalf("readmit: code = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	if m := ana.member(groupID, joined.MemberId.String()); m.Status != "former" {
		t.Errorf("Beto's member = %+v, want former until he comes back himself", m)
	}
	if again := beto.join(link); again.MemberId != joined.MemberId {
		t.Errorf("Beto came back as %s, want his former member %s", again.MemberId, joined.MemberId)
	}
	want := "group_created,member_joined,member_joined,member_removed,member_readmitted,member_joined"
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != want {
		t.Errorf("activities = %s, want %s", kinds, want)
	}
}

func TestReadmitRefusals(t *testing.T) {
	w := newWorld(t)
	ana, beto, carla, dani := w.signIn("Ana"), w.signIn("Beto"), w.signIn("Carla"), w.signIn("Dani")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Eva"}]}`)
	link := ana.groupLink(groupID)
	betoID := beto.join(link).MemberId.String()
	carlaID := carla.join(link).MemberId.String()
	carla.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")
	evaID := ana.memberID(groupID, "Eva")
	ana.removeFromGroup(groupID, evaID)
	daniID := dani.join(link).MemberId.String()
	ana.removeFromGroup(groupID, daniID)

	tests := []struct {
		name     string
		who      *person
		memberID string
		status   int
		code     httpapi.ProblemCode
	}{
		{"an active Member", ana, betoID, http.StatusConflict, httpapi.ProblemCodeMemberNotRemoved},
		{"who left by themselves", ana, carlaID, http.StatusConflict, httpapi.ProblemCodeMemberNotRemoved},
		{"a removed pending Member, whom no User can come back as", ana, evaID, http.StatusConflict, httpapi.ProblemCodeMemberNotRemoved},
		{"a Member that never existed", ana, someID, http.StatusNotFound, httpapi.ProblemCodeNotFound},
		{"from outside the Group", carla, daniID, http.StatusNotFound, httpapi.ProblemCodeNotFound},
		{"themselves", dani, daniID, http.StatusNotFound, httpapi.ProblemCodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.who.readmit(groupID, tt.memberID)

			decodeProblem(t, rec, tt.status, tt.code)
		})
	}
}

// Claiming another pending Member is refused to anyone already in the Group
// once, but a removed User must hear why.
func TestARemovedUserCannotClaimAPendingMemberEither(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Betico"}]}`)
	link := ana.groupLink(groupID)
	ana.removeFromGroup(groupID, beto.join(link).MemberId.String())

	rec := beto.claim(link, ana.memberID(groupID, "Betico"))

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeRemovedFromGroup)
}

// The invite page says it before the User picks anything, and only to them.
func TestAnInviteTellsARemovedUserTheyMustBeReadmitted(t *testing.T) {
	w := newWorld(t)
	ana, beto, carla := w.signIn("Ana"), w.signIn("Beto"), w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	link := ana.groupLink(groupID)
	betoID := beto.join(link).MemberId.String()
	carla.join(link)
	carla.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")
	ana.removeFromGroup(groupID, betoID)

	for _, tt := range []struct {
		name    string
		who     *person
		removed bool
	}{
		{"removed", beto, true},
		{"left by themselves", carla, false},
		{"still in the Group", ana, false},
		{"without a session", w.anonymous(), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			invite := decode[httpapi.Invite](t, tt.who.send(http.MethodGet, "/api/v1/invites/"+link, ""), http.StatusOK)

			if got := invite.Removed != nil && *invite.Removed; got != tt.removed {
				t.Errorf("removed = %v, want %v", got, tt.removed)
			}
		})
	}

	ana.readmit(groupID, betoID)
	if invite := decode[httpapi.Invite](t, beto.send(http.MethodGet, "/api/v1/invites/"+link, ""), http.StatusOK); invite.Removed != nil && *invite.Removed {
		t.Errorf("readmitted Beto still reads removed")
	}
}

// synced is what p's replica gets from GET /sync, from scratch.
func (p *person) synced() httpapi.Changes {
	p.w.t.Helper()
	return decode[httpapi.Changes](p.w.t, p.send(http.MethodGet, "/api/v1/sync?since=0", ""), http.StatusOK)
}

// syncedMember is memberID as p's replica gets it.
func (p *person) syncedMember(memberID string) httpapi.Member {
	p.w.t.Helper()
	for _, m := range p.synced().Members {
		if m.Id.String() == memberID {
			return m
		}
	}
	p.w.t.Fatalf("%s got no Member %s", p.name, memberID)
	return httpapi.Member{}
}

// The Group's settings list who waits to be readmitted.
func TestThePullSaysWhoWasRemoved(t *testing.T) {
	w := newWorld(t)
	ana, beto, carla := w.signIn("Ana"), w.signIn("Beto"), w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Eva"}]}`)
	link := ana.groupLink(groupID)
	betoID := beto.join(link).MemberId.String()
	carlaID := carla.join(link).MemberId.String()
	carla.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")
	evaID := ana.memberID(groupID, "Eva")
	ana.removeFromGroup(groupID, evaID)
	ana.removeFromGroup(groupID, betoID)

	for _, tt := range []struct {
		name     string
		memberID string
		removed  bool
	}{
		{"removed", betoID, true},
		{"left by themselves", carlaID, false},
		{"pending when removed", evaID, false},
		{"still in the Group", ana.memberID(groupID, "Ana"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := ana.syncedMember(tt.memberID)

			if got := m.Removed != nil && *m.Removed; got != tt.removed {
				t.Errorf("removed = %v, want %v", got, tt.removed)
			}
		})
	}

	ana.readmit(groupID, betoID)
	if m := ana.syncedMember(betoID); m.Removed != nil && *m.Removed {
		t.Errorf("readmitted Beto = %+v, want no longer removed", m)
	}
}

// Unlinking the former member would free the User to join as somebody new,
// without anyone readmitting them.
func TestARemovedUsersMemberCannotBeUnlinked(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	link := ana.groupLink(groupID)
	betoID := beto.join(link).MemberId.String()
	ana.removeFromGroup(groupID, betoID)

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+betoID+"/claim", "")

	decodeProblem(t, rec, http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decodeProblem(t, beto.send(http.MethodPost, "/api/v1/invites/"+link+"/join", ""), http.StatusConflict, httpapi.ProblemCodeRemovedFromGroup)
}
