package auth_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

// withMailBudget sets how many emails the server sends per hour and per day.
func withMailBudget(perHour, perDay int) func(*auth.Config) {
	return func(cfg *auth.Config) { cfg.MailPerHour, cfg.MailPerDay = perHour, perDay }
}

// logBuffer collects what the Service logs.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// errors returns the lines logged at ERROR level.
func (l *logBuffer) errors() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var lines []string
	for line := range strings.Lines(l.buf.String()) {
		if strings.Contains(line, "level=ERROR") {
			lines = append(lines, line)
		}
	}
	return lines
}

func withLog(l *logBuffer) func(*auth.Config) {
	return func(cfg *auth.Config) { cfg.Logger = slog.New(slog.NewTextHandler(l, nil)) }
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

// askForCodes asks for a code for n new emails, each from its own IP.
func (f *fixture) askForCodes(t *testing.T, n int) {
	t.Helper()
	for range n {
		f.asked++
		email, ip := fmt.Sprintf("user%d@example.com", f.asked), fmt.Sprintf("198.51.100.%d", f.asked%256)
		if err := f.at(ip).requestCode(email); err != nil {
			t.Fatalf("code %d: %v", f.asked, err)
		}
	}
}

// wantBudgetSpent checks that the next code is refused for the budget until
// retryAfter.
func (f *fixture) wantBudgetSpent(t *testing.T, retryAfter time.Duration) {
	t.Helper()
	err := f.at("192.0.2.1").requestCode("late@example.com")
	var spent *auth.MailBudgetError
	if !errors.As(err, &spent) || !errors.Is(err, auth.ErrMailBudgetSpent) || spent.RetryAfter != retryAfter {
		t.Fatalf("err = %v, want ErrMailBudgetSpent for %v", err, retryAfter)
	}
}

func TestTheServerStopsSendingAtItsBudgetForTheHour(t *testing.T) {
	var log logBuffer
	f := newFixture(t, withMailBudget(3, 10), withLog(&log))
	f.clock.Advance(20 * time.Minute)
	f.askForCodes(t, 3)
	sent := len(f.mail.sent)

	f.wantBudgetSpent(t, 40*time.Minute)
	f.wantBudgetSpent(t, 40*time.Minute)

	if len(f.mail.sent) != sent {
		t.Errorf("sent %d emails over the budget", len(f.mail.sent)-sent)
	}
	want := []string{"email budget is spent", "sent_this_hour=3 max_per_hour=3", "sent_last_24h=3 max_per_day=10", "resumes_at=2026-10-26T13:00:00Z"}
	if lines := log.errors(); len(lines) != 1 || !containsAll(lines[0], want) {
		t.Errorf("errors logged = %q, want one line that says the budget is spent with %q", lines, want)
	}

	f.clock.Advance(40 * time.Minute)
	f.askForCodes(t, 1)
}

func TestTheServerStopsSendingAtItsBudgetForTheDay(t *testing.T) {
	var log logBuffer
	f := newFixture(t, withMailBudget(3, 5), withLog(&log))
	f.askForCodes(t, 3) // 12:00, spends the hour
	f.clock.Advance(5 * time.Hour)
	f.askForCodes(t, 2) // 17:00, spends the day

	// The three emails of 12:00 leave the day at 12:00 tomorrow.
	f.wantBudgetSpent(t, 19*time.Hour)
	want := []string{"sent_this_hour=2 max_per_hour=3", "sent_last_24h=5 max_per_day=5", "resumes_at=2026-10-27T12:00:00Z"}
	if lines := log.errors(); len(lines) != 2 || !containsAll(lines[1], want) {
		t.Errorf("errors logged = %q, want one for the hour and one for the day with %q", lines, want)
	}

	f.clock.Advance(19 * time.Hour)
	f.askForCodes(t, 3)
	// The hour is spent too, but the two emails of 17:00 leave later.
	f.wantBudgetSpent(t, 5*time.Hour)
}

// Every deploy restarts the server.
func TestARestartDoesNotForgetTheEmailsSent(t *testing.T) {
	f := newFixture(t, withMailBudget(3, 10))
	f.askForCodes(t, 3)

	restarted, err := auth.New(auth.Config{DB: f.db, Mail: f.mail, DefaultCurrency: "USD", Now: f.clock.Now, MailPerHour: 3, MailPerDay: 10})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.at("192.0.2.1").requestCodeFrom(restarted, "late@example.com"); !errors.Is(err, auth.ErrMailBudgetSpent) {
		t.Errorf("err = %v, want ErrMailBudgetSpent", err)
	}
}

func TestAnEmailThatFailedDoesNotSpendTheBudget(t *testing.T) {
	f := newFixture(t, withMailBudget(3, 10), func(cfg *auth.Config) { cfg.Mail = failingMail{} })
	for i := range 5 {
		if err := f.at("192.0.2.1").requestCode(fmt.Sprintf("user%d@example.com", i)); err == nil || errors.Is(err, auth.ErrMailBudgetSpent) {
			t.Fatalf("request %d: err = %v, want the mail failure", i+1, err)
		}
	}
}

// Nobody could have served it, so the sender keeps its quota for later.
func TestARequestRefusedForTheBudgetCostsTheSenderNothing(t *testing.T) {
	f := newFixture(t, withMailBudget(1, 10))
	f.askForCodes(t, 1)
	for range 20 {
		if err := f.at(anaIP).requestCode(anaEmail); !errors.Is(err, auth.ErrMailBudgetSpent) {
			t.Fatalf("err = %v, want ErrMailBudgetSpent", err)
		}
	}

	f.clock.Advance(time.Hour)

	if err := f.at(anaIP).requestCode(anaEmail); err != nil {
		t.Errorf("an hour later: %v", err)
	}
}

// Requests at once can all pass the look at the budget and race for its last
// email. Those that lose it under the write lock cost nothing either.
func TestRequestsThatLoseTheLastEmailCostTheSenderNothing(t *testing.T) {
	f := newFixture(t, withMailBudget(1, 10))
	f.clock.Advance(50 * time.Minute)
	results := make(chan error, 20)
	for range 20 {
		go func() { results <- f.newBrowser(anaIP).requestCode(anaEmail) }()
	}
	sent := 0
	for range 20 {
		switch err := <-results; {
		case err == nil:
			sent++
		case !errors.Is(err, auth.ErrMailBudgetSpent):
			t.Errorf("err = %v, want ErrMailBudgetSpent", err)
		}
	}
	if sent != 1 {
		t.Fatalf("sent %d codes, want the 1 of the budget", sent)
	}

	f.clock.Advance(10 * time.Minute) // a new hour of the budget

	if err := f.newBrowser(anaIP).requestCode(anaEmail); err != nil {
		t.Errorf("err = %v, want the second of the 3 codes an hour of the IP for Ana", err)
	}
}

func TestTheBudgetIsFortyEmailsAnHourByDefault(t *testing.T) {
	f := newFixture(t)
	f.askForCodes(t, 40)

	f.wantBudgetSpent(t, time.Hour)
}
