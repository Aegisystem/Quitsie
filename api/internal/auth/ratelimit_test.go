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
	// More than an IP gets in an hour: they do not count against it either.
	for i := range 12 {
		if err := f.at(anaIP).requestCodeNow(anaEmail); err == nil || errors.Is(err, auth.ErrRateLimited) {
			t.Fatalf("attempt %d while the email service refuses: err = %v, want its failure", i+1, err)
		}
	}

	broken = false
	if err := f.at(anaIP).requestCodeNow(anaEmail); err != nil {
		t.Errorf("once the email service works: %v, want a code at once", err)
	}
}

// Each code is an email out of the server's budget, which one address could
// otherwise spend alone by asking for many emails.
func TestAnIPGetsTenCodesPerHour(t *testing.T) {
	f := newFixture(t)
	for i := range 10 {
		if err := f.newBrowser(anaIP).requestCodeNow(fmt.Sprintf("user%d@example.com", i)); err != nil {
			t.Fatalf("code %d: %v", i+1, err)
		}
	}
	f.clock.Advance(20 * time.Minute)

	err := f.newBrowser(anaIP).requestCodeNow("late@example.com")
	var limited *auth.RateLimitError
	if !errors.As(err, &limited) || limited.RetryAfter != 40*time.Minute {
		t.Fatalf("code 11, 20 minutes later: err = %v, want a wait of 40 minutes", err)
	}
	// The refusal sent nothing, so that email does not wait.
	if err := f.newBrowser("198.51.100.1").requestCodeNow("late@example.com"); err != nil {
		t.Errorf("the same email from another IP, at once: %v", err)
	}

	f.clock.Advance(40 * time.Minute)
	if err := f.newBrowser(anaIP).requestCodeNow("later@example.com"); err != nil {
		t.Errorf("an hour after the ten: %v", err)
	}
	if len(f.mail.sent) != 12 {
		t.Errorf("sent %d emails, want 12", len(f.mail.sent))
	}
}

// Otherwise one browser that signed in once could spend the budget alone,
// with a code per minute for its own email.
func TestAKnownDeviceCountsAgainstItsIPToo(t *testing.T) {
	f := newFixture(t)
	phone := f.signedInBrowser(t, anaEmail, anaIP)
	for i := range 9 {
		if err := phone.requestCode(anaEmail); err != nil {
			t.Fatalf("code %d: %v", i+2, err)
		}
	}

	if err := phone.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("code 11 within the hour: err = %v, want ErrRateLimited", err)
	}
}

// Only a code that went out counts against the IP: someone who asks again too
// soon, as a person does when the email is slow, loses nothing.
func TestARefusedRequestDoesNotCountAgainstTheIP(t *testing.T) {
	f := newFixture(t)
	if err := f.newBrowser(anaIP).requestCodeNow(anaEmail); err != nil {
		t.Fatal(err)
	}
	for range 15 {
		if err := f.newBrowser(anaIP).requestCodeNow(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
			t.Fatalf("again within the minute: err = %v, want ErrRateLimited", err)
		}
	}

	for i := range 9 {
		if err := f.newBrowser(anaIP).requestCodeNow(fmt.Sprintf("user%d@example.com", i)); err != nil {
			t.Fatalf("code %d of the IP: %v", i+2, err)
		}
	}
}

// Requests of one IP at once can all pass the look at its codes and race for
// the last one. Those that lose it sent nothing, so their emails do not wait.
func TestRequestsThatLoseTheLastCodeOfAnIPDoNotMakeTheirEmailsWait(t *testing.T) {
	f := newFixture(t)
	for i := range 9 {
		if err := f.newBrowser(anaIP).requestCodeNow(fmt.Sprintf("user%d@example.com", i)); err != nil {
			t.Fatalf("code %d: %v", i+1, err)
		}
	}
	type result struct {
		email string
		err   error
	}
	results := make(chan result, 20)
	for i := range 20 {
		email := fmt.Sprintf("racer%d@example.com", i)
		go func() { results <- result{email, f.newBrowser(anaIP).requestCodeNow(email)} }()
	}
	var lost []string
	for range 20 {
		switch r := <-results; {
		case r.err == nil:
		case errors.Is(r.err, auth.ErrRateLimited):
			lost = append(lost, r.email)
		default:
			t.Errorf("err = %v, want ErrRateLimited", r.err)
		}
	}
	if len(lost) != 19 {
		t.Fatalf("sent %d codes, want the 1 the IP had left", 20-len(lost))
	}

	for i, email := range lost {
		if err := f.newBrowser(fmt.Sprintf("198.51.100.%d", i)).requestCodeNow(email); err != nil {
			t.Errorf("%s from another IP, at once: err = %v, want a code", email, err)
		}
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
