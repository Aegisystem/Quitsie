package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

const someID = "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b"

// operationRequest is one request that reaches an operation.
type operationRequest struct {
	op     string
	method string
	target string
	body   string
}

// The operations that still answer 501, one list per area of the API so that
// the work on each area does not collide. When an operation is built, move its
// row to a test of its own and its name to the matching implemented list.
var unimplemented = slices.Concat(unimplementedAuth, unimplementedLedgers, unimplementedRecords)

// Sign-in, sessions and the profile.
var unimplementedAuth = []operationRequest{
	{"RequestLoginCode", http.MethodPost, "/api/v1/auth/login-code", "{}"},
	{"VerifyLoginCode", http.MethodPost, "/api/v1/auth/login-code/verify", "{}"},
	{"StartGoogleLogin", http.MethodGet, "/api/v1/auth/google?returnTo=/groups", ""},
	{"FinishGoogleLogin", http.MethodGet, "/api/v1/auth/google/callback?code=c&state=s", ""},
	{"Logout", http.MethodPost, "/api/v1/auth/logout", ""},
	{"ListSessions", http.MethodGet, "/api/v1/auth/sessions", ""},
	{"RevokeSession", http.MethodDelete, "/api/v1/auth/sessions/" + someID, ""},
	{"GetMe", http.MethodGet, "/api/v1/me", ""},
	{"UpdateMe", http.MethodPatch, "/api/v1/me", "{}"},
	{"DeleteMe", http.MethodDelete, "/api/v1/me", ""},
}

// Groups, Members, invitations and Friendships.
var unimplementedLedgers = []operationRequest{
	{"UndoClaim", http.MethodDelete, "/api/v1/groups/" + someID + "/members/" + someID + "/claim", ""},
	{"RegenerateGroupInviteLink", http.MethodPost, "/api/v1/groups/" + someID + "/invite-link", ""},
	{"GetInvite", http.MethodGet, "/api/v1/invites/k3J9xQ2mV7pL8wR4tY6u", ""},
	{"ClaimMember", http.MethodPost, "/api/v1/invites/k3J9xQ2mV7pL8wR4tY6u/claim", "{}"},
	{"JoinGroup", http.MethodPost, "/api/v1/invites/k3J9xQ2mV7pL8wR4tY6u/join", ""},
	{"CreateFriendship", http.MethodPost, "/api/v1/friendships", "{}"},
	{"RegenerateFriendInviteLink", http.MethodPost, "/api/v1/friendships/" + someID + "/invite-link", ""},
}

// Expenses, Settlements, records and sync.
var unimplementedRecords = []operationRequest{
	{"PushRecords", http.MethodPost, "/api/v1/records", `{"expenses":[],"settlements":[]}`},
	{"UpdateExpense", http.MethodPatch, "/api/v1/expenses/" + someID, "{}"},
	{"DeleteExpense", http.MethodDelete, "/api/v1/expenses/" + someID, ""},
	{"RestoreExpense", http.MethodPost, "/api/v1/expenses/" + someID + "/restore", ""},
	{"UpdateSettlement", http.MethodPatch, "/api/v1/settlements/" + someID, "{}"},
	{"DeleteSettlement", http.MethodDelete, "/api/v1/settlements/" + someID, ""},
	{"RestoreSettlement", http.MethodPost, "/api/v1/settlements/" + someID + "/restore", ""},
	{"PullChanges", http.MethodGet, "/api/v1/sync?since=0", ""},
}

// The operations that have a real handler, per area.
var (
	implementedBase    = []string{"GetHealth"}
	implementedAuth    = []string{}
	implementedLedgers = []string{"CreateGroup", "UpdateGroup", "DeleteGroup", "AddPendingMember", "RemoveMember", "LeaveGroup"}
	implementedRecords = []string{}
	implemented        = slices.Concat(implementedBase, implementedAuth, implementedLedgers, implementedRecords)
)

func send(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeProblem checks that rec is an application/problem+json body with the
// given status and code.
func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code httpapi.ProblemCode) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("code = %d, want %d (body %q)", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p httpapi.Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Code != code || p.Status != status || p.Title == "" {
		t.Errorf("problem = %+v, want code %q and status %d with a title", p, code, status)
	}
}

func TestUnimplementedOperationsAnswer501(t *testing.T) {
	h := httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test"), builtPWA)

	for _, tt := range unimplemented {
		t.Run(tt.op, func(t *testing.T) {
			rec := send(t, h, tt.method, tt.target, tt.body)

			decodeProblem(t, rec, http.StatusNotImplemented, httpapi.ProblemCodeNotImplemented)
		})
	}
}

func TestEveryOperationIsListed(t *testing.T) {
	listed := slices.Clone(implemented)
	for _, tt := range unimplemented {
		listed = append(listed, tt.op)
	}
	ops := reflect.TypeFor[httpapi.StrictServerInterface]()
	for i := range ops.NumMethod() {
		if name := ops.Method(i).Name; !slices.Contains(listed, name) {
			t.Errorf("operation %s is in openapi.yaml but in neither list of this test", name)
		}
	}
	if len(listed) != ops.NumMethod() {
		t.Errorf("the lists name %d operations, StrictServerInterface has %d", len(listed), ops.NumMethod())
	}
}

func TestRequestsThatBreakTheContract(t *testing.T) {
	h := httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test"), builtPWA)

	tests := []struct {
		name   string
		method string
		target string
		body   string
		status int
		code   httpapi.ProblemCode
	}{
		{"path id is not a UUID", http.MethodDelete, "/api/v1/expenses/not-a-uuid", "", http.StatusBadRequest, httpapi.ProblemCodeValidationFailed},
		{"sync without a cursor", http.MethodGet, "/api/v1/sync", "", http.StatusBadRequest, httpapi.ProblemCodeValidationFailed},
		{"sync cursor is not a number", http.MethodGet, "/api/v1/sync?since=abc", "", http.StatusBadRequest, httpapi.ProblemCodeValidationFailed},
		{"body is not JSON", http.MethodPost, "/api/v1/records", "{", http.StatusBadRequest, httpapi.ProblemCodeValidationFailed},
		{"required body is missing", http.MethodPost, "/api/v1/groups", "", http.StatusBadRequest, httpapi.ProblemCodeValidationFailed},
		{"unknown API route", http.MethodGet, "/api/v1/nope", "", http.StatusNotFound, httpapi.ProblemCodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, h, tt.method, tt.target, tt.body)

			decodeProblem(t, rec, tt.status, tt.code)
		})
	}
}
