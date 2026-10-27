package auth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

func TestAnEmailGetsAtMostFiveCodesPerHour(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		f.clock.Advance(time.Minute)
	}

	if err := f.svc.RequestCode(ctx, anaEmail, "198.51.100.1"); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("sixth request within the hour, even from another IP: err = %v, want ErrRateLimited", err)
	}
	if len(f.mail.sent) != 5 {
		t.Errorf("sent %d emails, want 5", len(f.mail.sent))
	}

	f.clock.Advance(56 * time.Minute) // the first request is now an hour old
	if err := f.svc.RequestCode(ctx, anaEmail, anaIP); err != nil {
		t.Errorf("request an hour after the first: %v", err)
	}
}

func TestAnIPGetsAtMostThirtyCodesPerHour(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := 1; i <= 30; i++ {
		if err := f.svc.RequestCode(ctx, fmt.Sprintf("user%d@example.com", i), anaIP); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	if err := f.svc.RequestCode(ctx, "user31@example.com", anaIP); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("31st request from one IP: err = %v, want ErrRateLimited", err)
	}
	if err := f.svc.RequestCode(ctx, "user31@example.com", "198.51.100.1"); err != nil {
		t.Errorf("the same email from another IP: %v", err)
	}
}
