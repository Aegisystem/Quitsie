package mail_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/mail"
)

// fakeResend stands in for https://api.resend.com and records the last email it got.
type fakeResend struct {
	status int
	reply  string

	method, path, auth, contentType string
	body                            map[string]any
}

func (f *fakeResend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.method, f.path = r.Method, r.URL.Path
	f.auth, f.contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
	f.body = nil
	_ = json.NewDecoder(r.Body).Decode(&f.body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.reply))
}

func newResend(t *testing.T, fake *fakeResend) *mail.Resend {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	r, err := mail.NewResend(mail.ResendConfig{
		APIKey:  "re_test_key",
		From:    "Quitsie <codigos@example.com>",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResendPostsTheEmailToTheAPI(t *testing.T) {
	fake := &fakeResend{status: http.StatusOK, reply: `{"id":"49a3999c-0ce1-4ea6-ab68-afcd6dc2e794"}`}
	r := newResend(t, fake)

	err := r.Send(context.Background(), mail.Message{
		To:      "ana@example.com",
		Subject: "Tu código de Quitsie: 123456",
		Text:    "Tu código es 123456.",
	})
	if err != nil {
		t.Fatal(err)
	}

	if fake.method != http.MethodPost || fake.path != "/emails" {
		t.Errorf("request = %s %s, want POST /emails", fake.method, fake.path)
	}
	if fake.auth != "Bearer re_test_key" {
		t.Errorf("Authorization = %q", fake.auth)
	}
	if fake.contentType != "application/json" {
		t.Errorf("Content-Type = %q", fake.contentType)
	}
	want := map[string]any{
		"from":    "Quitsie <codigos@example.com>",
		"to":      []any{"ana@example.com"},
		"subject": "Tu código de Quitsie: 123456",
		"text":    "Tu código es 123456.",
	}
	if got, _ := json.Marshal(fake.body); string(got) != mustJSON(t, want) {
		t.Errorf("body = %s, want %s", got, mustJSON(t, want))
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestResendReportsRejectedEmails(t *testing.T) {
	fake := &fakeResend{status: http.StatusForbidden, reply: `{"statusCode":403,"name":"validation_error","message":"The example.com domain is not verified."}`}
	r := newResend(t, fake)

	err := r.Send(context.Background(), mail.Message{To: "ana@example.com", Subject: "s", Text: "t"})
	if err == nil || !strings.Contains(err.Error(), "domain is not verified") {
		t.Errorf("Send error = %v, want Resend's message", err)
	}
}

func TestResendNeedsAKeyAndASender(t *testing.T) {
	for _, cfg := range []mail.ResendConfig{
		{From: "Quitsie <codigos@example.com>"},
		{APIKey: "re_test_key"},
	} {
		if _, err := mail.NewResend(cfg); err == nil {
			t.Errorf("NewResend(%+v) accepted an incomplete configuration", cfg)
		}
	}
}
