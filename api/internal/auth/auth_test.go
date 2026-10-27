package auth_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// outbox is a mail.Sender that keeps the emails instead of sending them.
type outbox struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (o *outbox) Send(_ context.Context, m mail.Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent = append(o.sent, m)
	return nil
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

// lastCode returns the login code in the last email sent to `to`.
func (o *outbox) lastCode(t *testing.T, to string) string {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := len(o.sent) - 1; i >= 0; i-- {
		if o.sent[i].To == to {
			code := sixDigits.FindString(o.sent[i].Text)
			if code == "" {
				t.Fatalf("email to %s has no 6-digit code: %q", to, o.sent[i].Text)
			}
			return code
		}
	}
	t.Fatalf("no email sent to %s", to)
	return ""
}

// clock is a settable time source.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	svc   *auth.Service
	db    *sql.DB
	mail  *outbox
	clock *clock
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "quitsie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	f := &fixture{
		db:    db,
		mail:  &outbox{},
		clock: &clock{now: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)},
	}
	f.svc, err = auth.New(auth.Config{
		DB:              db,
		Mail:            f.mail,
		DefaultCurrency: "USD",
		Now:             f.clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

const (
	anaEmail = "ana@example.com"
	anaIP    = "203.0.113.7"
	device   = "Firefox on Linux"

	codeLifetime = 10 * time.Minute
)

func TestARequestedCodeSignsANewUserIn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	signIn, err := f.svc.VerifyCode(ctx, anaEmail, f.mail.lastCode(t, anaEmail), device)
	if err != nil {
		t.Fatal(err)
	}

	if signIn.User.Email != anaEmail || signIn.User.ID == "" {
		t.Errorf("User = %+v, want a new User for %s", signIn.User, anaEmail)
	}
	if !signIn.NewUser {
		t.Error("NewUser = false for an email never seen before")
	}
	if signIn.User.PreferredCurrency != "USD" {
		t.Errorf("PreferredCurrency = %q, want the default USD", signIn.User.PreferredCurrency)
	}
	if signIn.Token == "" {
		t.Error("no session token")
	}
}

// wrongCode returns a 6-digit code that is not code.
func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func TestAWrongCodeIsRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)

	if _, err := f.svc.VerifyCode(ctx, anaEmail, wrongCode(code), device); !errors.Is(err, auth.ErrWrongCode) {
		t.Fatalf("VerifyCode with a wrong code: err = %v, want ErrWrongCode", err)
	}
	if _, err := f.svc.VerifyCode(ctx, anaEmail, code, device); err != nil {
		t.Errorf("the right code after a wrong one: %v", err)
	}
}

func TestACodeExpiresAfterTenMinutes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)

	f.clock.Advance(10 * time.Minute)
	if _, err := f.svc.VerifyCode(ctx, anaEmail, code, device); !errors.Is(err, auth.ErrCodeExpired) {
		t.Errorf("VerifyCode 10 minutes later: err = %v, want ErrCodeExpired", err)
	}
}

func TestACodeStillWorksJustBeforeItExpires(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}

	f.clock.Advance(10*time.Minute - time.Second)
	if _, err := f.svc.VerifyCode(ctx, anaEmail, f.mail.lastCode(t, anaEmail), device); err != nil {
		t.Errorf("VerifyCode 9:59 later: %v", err)
	}
}

func TestACodeWorksOnlyOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)
	if _, err := f.svc.VerifyCode(ctx, anaEmail, code, device); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.VerifyCode(ctx, anaEmail, code, device); !errors.Is(err, auth.ErrCodeExpired) {
		t.Errorf("reusing a code: err = %v, want ErrCodeExpired", err)
	}
}

func TestVerifyingWithoutRequestingFails(t *testing.T) {
	f := newFixture(t)

	if _, err := f.svc.VerifyCode(context.Background(), anaEmail, "123456", device); !errors.Is(err, auth.ErrCodeExpired) {
		t.Errorf("err = %v, want ErrCodeExpired", err)
	}
}

func TestACodeAllowsFiveAttempts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)

	for i := 1; i <= 4; i++ {
		if _, err := f.svc.VerifyCode(ctx, anaEmail, wrongCode(code), device); !errors.Is(err, auth.ErrWrongCode) {
			t.Fatalf("attempt %d: err = %v, want ErrWrongCode", i, err)
		}
	}
	if _, err := f.svc.VerifyCode(ctx, anaEmail, code, device); err != nil {
		t.Errorf("fifth attempt with the right code: %v", err)
	}
}

func TestACodeIsLockedAfterFiveWrongAttempts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)

	for i := 1; i <= 5; i++ {
		if _, err := f.svc.VerifyCode(ctx, anaEmail, wrongCode(code), device); !errors.Is(err, auth.ErrWrongCode) {
			t.Fatalf("attempt %d: err = %v, want ErrWrongCode", i, err)
		}
	}
	if _, err := f.svc.VerifyCode(ctx, anaEmail, code, device); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Errorf("sixth attempt with the right code: err = %v, want ErrTooManyAttempts", err)
	}
}

func TestANewCodeReplacesTheEarlierOne(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	first := f.mail.lastCode(t, anaEmail)
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	second := f.mail.lastCode(t, anaEmail)
	if first == second {
		t.Skip("both codes came out equal (one chance in a million)")
	}

	if _, err := f.svc.VerifyCode(ctx, anaEmail, first, device); !errors.Is(err, auth.ErrWrongCode) {
		t.Errorf("the earlier code: err = %v, want ErrWrongCode", err)
	}
	if _, err := f.svc.VerifyCode(ctx, anaEmail, second, device); err != nil {
		t.Errorf("the newer code: %v", err)
	}
}

// signInWithCode requests a code for email and verifies it.
func (f *fixture) signInWithCode(t *testing.T, email, ip string) auth.SignIn {
	t.Helper()
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, email, ip); err != nil {
		t.Fatal(err)
	}
	signIn, err := f.svc.VerifyCode(ctx, email, f.mail.lastCode(t, email), device)
	if err != nil {
		t.Fatal(err)
	}
	return signIn
}

func TestAnExistingUserSignsInAgain(t *testing.T) {
	f := newFixture(t)
	first := f.signInWithCode(t, anaEmail, anaIP)

	again := f.signInWithCode(t, anaEmail, anaIP)

	if again.User.ID != first.User.ID {
		t.Errorf("User.ID = %s, want the existing %s", again.User.ID, first.User.ID)
	}
	if again.NewUser {
		t.Error("NewUser = true for a returning User")
	}
	if again.Token == first.Token {
		t.Error("the second sign-in reused the first session token")
	}
}

func TestEmailsIgnoreCaseAndSpaces(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first := f.signInWithCode(t, anaEmail, anaIP)

	if err := f.svc.RequestCode(ctx, "  Ana@Example.COM ", anaIP); err != nil {
		t.Fatal(err)
	}
	signIn, err := f.svc.VerifyCode(ctx, "ANA@example.com", f.mail.lastCode(t, anaEmail), device)
	if err != nil {
		t.Fatal(err)
	}
	if signIn.User.ID != first.User.ID {
		t.Errorf("signed in as %s, want %s", signIn.User.ID, first.User.ID)
	}
}

func TestInvalidEmailsGetNoCode(t *testing.T) {
	f := newFixture(t)

	for _, email := range []string{"", "ana", "ana@", "Ana <ana@example.com>", "ana@example.com, beto@example.com"} {
		if err := f.svc.RequestCode(context.Background(), email, anaIP); !errors.Is(err, auth.ErrInvalidEmail) {
			t.Errorf("RequestCode(%q): err = %v, want ErrInvalidEmail", email, err)
		}
	}
	if len(f.mail.sent) != 0 {
		t.Errorf("sent %d emails to invalid addresses", len(f.mail.sent))
	}
}

// dump returns every value in table as text, to check secrets never reach the database.
func (f *fixture) dump(t *testing.T, table string) string {
	t.Helper()
	rows, err := f.db.Query("SELECT * FROM " + table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out strings.Builder
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&out, vals...)
	}
	return out.String()
}

// The architecture's security section: only hashes of codes and tokens are stored.
func TestTheDatabaseKeepsOnlyHashes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)
	if strings.Contains(f.dump(t, "login_codes"), code) {
		t.Errorf("login_codes holds the code %s", code)
	}

	signIn, err := f.svc.VerifyCode(ctx, anaEmail, code, device)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.dump(t, "sessions"), signIn.Token) {
		t.Error("sessions holds the session token")
	}
}

// Emails of people who never finished signing in should not stay around.
func TestExpiredCodesAreForgotten(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, "beto@example.com", anaIP); err != nil {
		t.Fatal(err)
	}

	f.clock.Advance(codeLifetime)
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.dump(t, "login_codes"), "beto@example.com") {
		t.Error("login_codes still holds an expired code for beto@example.com")
	}
}
