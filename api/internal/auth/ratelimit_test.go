package auth_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

func TestAnEmailGetsAtMostFiveCodesPerHour(t *testing.T) {
	f := newFixture(t)
	for i := 1; i <= 5; i++ {
		if err := f.newBrowser(fmt.Sprintf("198.51.100.%d", i)).requestCode(anaEmail); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		f.clock.Advance(3 * time.Minute)
	}

	if err := f.newBrowser("198.51.100.6").requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("sixth request within the hour, from yet another IP: err = %v, want ErrRateLimited", err)
	}
	if len(f.mail.sent) != 5 {
		t.Errorf("sent %d emails, want 5", len(f.mail.sent))
	}

	f.clock.Advance(46 * time.Minute) // the first request is now an hour old
	if err := f.newBrowser("198.51.100.6").requestCode(anaEmail); err != nil {
		t.Errorf("request an hour after the first: %v", err)
	}
}

// One sender cannot take all the codes of an email from someone at another
// address.
func TestAnIPGetsAtMostThreeCodesPerHourForOneEmail(t *testing.T) {
	f := newFixture(t)
	for i := 1; i <= 3; i++ {
		if err := f.newBrowser(anaIP).requestCode(anaEmail); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	if err := f.newBrowser(anaIP).requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("fourth request from one IP: err = %v, want ErrRateLimited", err)
	}
	if err := f.newBrowser("198.51.100.1").requestCode(anaEmail); err != nil {
		t.Errorf("the same email from another IP: %v", err)
	}
	if err := f.newBrowser(anaIP).requestCode("beto@example.com"); err != nil {
		t.Errorf("another email from the same IP: %v", err)
	}
}

func TestAnIPGetsAtMostTenCodesPerHour(t *testing.T) {
	f := newFixture(t)
	for i := 1; i <= 10; i++ {
		if err := f.at(anaIP).requestCode(fmt.Sprintf("user%d@example.com", i)); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	if err := f.at(anaIP).requestCode("user11@example.com"); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("11th request from one IP: err = %v, want ErrRateLimited", err)
	}
	if err := f.at("198.51.100.1").requestCode("user11@example.com"); err != nil {
		t.Errorf("the same email from another IP: %v", err)
	}
}

// Requests of one sender at once still get no more than its quota.
func TestRequestsAtOnceGetNoMoreThanTheQuota(t *testing.T) {
	f := newFixture(t)
	results := make(chan error, 20)
	for i := range 20 {
		go func() { results <- f.newBrowser(anaIP).requestCode(fmt.Sprintf("user%d@example.com", i)) }()
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
	if sent != 10 {
		t.Errorf("sent %d codes, want the 10 of an IP", sent)
	}
}

// After a restart, which forgets the quotas, the live codes of an email cap
// its codes. A request refused there sends nothing, so it must not spend its
// sender's quotas either.
func TestARequestRefusedForTheLiveCodesCostsTheSenderNothing(t *testing.T) {
	f := newFixture(t)
	for i := range 5 {
		if err := f.newBrowser(fmt.Sprintf("198.51.100.%d", i)).requestCode(anaEmail); err != nil {
			t.Fatal(err)
		}
	}
	restarted, err := auth.New(auth.Config{DB: f.db, Mail: f.mail, DefaultCurrency: "USD", Now: f.clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if err := f.newBrowser(anaIP).requestCodeFrom(restarted, anaEmail); !errors.Is(err, auth.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	}

	f.clock.Advance(codeLifetime)

	// All 10 of the IP, the first one for Ana under the quota of the pair.
	for i := range 10 {
		email := fmt.Sprintf("user%d@example.com", i)
		if i == 0 {
			email = anaEmail
		}
		if err := f.newBrowser(anaIP).requestCodeFrom(restarted, email); err != nil {
			t.Fatalf("code %d once Ana's codes expired: %v", i+1, err)
		}
	}
}
