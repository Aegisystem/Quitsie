package auth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/mail"
)

// An email waits a minute between codes, whoever asks for them.
func TestAnEmailWaitsAMinuteBetweenCodes(t *testing.T) {
	f := newFixture(t)
	if err := f.newBrowser(anaIP).requestCodeNow(anaEmail); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(20 * time.Second)

	err := f.newBrowser("198.51.100.1").requestCodeNow(anaEmail)
	var limited *auth.RateLimitError
	if !errors.As(err, &limited) || limited.RetryAfter != 40*time.Second {
		t.Fatalf("a second code 20 s later, from another IP: err = %v, want a wait of 40 s", err)
	}
	if err := f.newBrowser(anaIP).requestCodeNow("beto@example.com"); err != nil {
		t.Errorf("another email at once: %v", err)
	}

	f.clock.Advance(40 * time.Second)
	if err := f.newBrowser(anaIP).requestCodeNow(anaEmail); err != nil {
		t.Errorf("a minute after the first: %v", err)
	}
	if len(f.mail.sent) != 3 {
		t.Errorf("sent %d emails, want 3", len(f.mail.sent))
	}
}

// What kept someone out for an hour: each attempt the email service refused
// used to count.
func TestACodeThatCouldNotBeSentMakesNobodyWait(t *testing.T) {
	broken := true
	f := newFixture(t, func(cfg *auth.Config) {
		cfg.Mail = switchableMail{broken: &broken, outbox: cfg.Mail.(*outbox)}
	})
	for i := range 5 {
		if err := f.at(anaIP).requestCodeNow(anaEmail); err == nil || errors.Is(err, auth.ErrRateLimited) {
			t.Fatalf("attempt %d while the email service refuses: err = %v, want its failure", i+1, err)
		}
	}

	broken = false
	if err := f.at(anaIP).requestCodeNow(anaEmail); err != nil {
		t.Errorf("once the email service works: %v, want a code at once", err)
	}
}

// Requests for one email at once still get a single code.
func TestRequestsAtOnceGetOneCode(t *testing.T) {
	f := newFixture(t)
	results := make(chan error, 20)
	for i := range 20 {
		go func() { results <- f.newBrowser(fmt.Sprintf("198.51.100.%d", i)).requestCodeNow(anaEmail) }()
	}

	sent := 0
	for range 20 {
		switch err := <-results; {
		case err == nil:
			sent++
		case !errors.Is(err, auth.ErrRateLimited):
			t.Errorf("err = %v, want ErrRateLimited", err)
		}
	}
	if sent != 1 {
		t.Errorf("sent %d codes, want 1", sent)
	}
}

// After a restart, which forgets the waits, the live codes of an email cap
// its codes. A request refused there sends nothing, so it must not make its
// sender wait either.
func TestARequestRefusedForTheLiveCodesCostsTheSenderNothing(t *testing.T) {
	f := newFixture(t)
	for i := range 10 {
		if err := f.newBrowser(fmt.Sprintf("198.51.100.%d", i)).requestCodeNow(anaEmail); err != nil {
			t.Fatalf("code %d: %v", i+1, err)
		}
		f.clock.Advance(time.Second)
		restarted, err := auth.New(auth.Config{DB: f.db, Mail: f.mail, DefaultCurrency: "USD", Now: f.clock.Now})
		if err != nil {
			t.Fatal(err)
		}
		f.svc = restarted
	}
	for range 3 {
		if err := f.newBrowser(anaIP).requestCodeNow(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	}

	f.clock.Advance(codeLifetime)

	if err := f.newBrowser(anaIP).requestCodeNow(anaEmail); err != nil {
		t.Errorf("once Ana's codes expired: %v", err)
	}
}

// switchableMail refuses every email while broken, like Resend refusing a
// recipient, and delivers to outbox otherwise.
type switchableMail struct {
	broken *bool
	outbox *outbox
}

func (m switchableMail) Send(ctx context.Context, msg mail.Message) error {
	if *m.broken {
		return errors.New("resend: 403 Forbidden")
	}
	return m.outbox.Send(ctx, msg)
}
