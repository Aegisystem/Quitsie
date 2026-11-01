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
