package auth_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

// The three ways of finding 5 of docs/revision-seguridad.md to keep someone
// from signing in, which only need their email, and the guesses of finding 4,
// which must stay capped.

const attackerIP = "198.51.100.66"

// signedInBrowser returns a new browser at ip that already signed in to
// email with a code, as a phone that was used before.
func (f *fixture) signedInBrowser(t *testing.T, email, ip string) *browser {
	t.Helper()
	b := f.newBrowser(ip)
	if err := b.requestCode(email); err != nil {
		t.Fatal(err)
	}
	if _, err := b.verify(email, f.mail.lastCode(t, email)); err != nil {
		t.Fatal(err)
	}
	return b
}

// signIn asks for a code for email and checks it, from b.
func (b *browser) signIn(t *testing.T, mail *outbox, email string) error {
	t.Helper()
	if err := b.requestCode(email); err != nil {
		return err
	}
	_, err := b.verify(email, mail.lastCode(t, email))
	return err
}

// guessWrongFrom spends wrong guesses at email's codes from new browsers at
// ip, n of them per code, until the server stops it. It returns how many
// guesses got in.
func (f *fixture) guessWrongFrom(t *testing.T, email string, ips []string, perCode int) int {
	t.Helper()
	guesses := 0
	for _, ip := range ips {
		b := f.newBrowser(ip)
		if err := b.requestCode(email); err != nil {
			if !errors.Is(err, auth.ErrRateLimited) {
				t.Fatalf("request from %s: %v", ip, err)
			}
			continue
		}
		code := f.mail.lastCode(t, email)
		for range perCode {
			_, err := b.verify(email, wrongCode(code))
			if errors.Is(err, auth.ErrTooManyAttempts) {
				break
			}
			if !errors.Is(err, auth.ErrWrongCode) {
				t.Fatalf("guess from %s: err = %v, want ErrWrongCode", ip, err)
			}
			guesses++
		}
	}
	return guesses
}

// Way 1: five requests used to spend the codes of an email for an hour.
func TestSpendingTheCodesOfAnEmailDoesNotStopItsKnownDevice(t *testing.T) {
	f := newFixture(t)
	phone := f.signedInBrowser(t, anaEmail, anaIP)
	f.clock.Advance(time.Hour)
	attacker := 0
	for i := range 10 {
		if err := f.newBrowser(fmt.Sprintf("198.51.100.%d", i)).requestCode(anaEmail); err == nil {
			attacker++
		}
	}
	if attacker != 5 {
		t.Fatalf("the attacker got %d codes, want the 5 of an email per hour", attacker)
	}

	if err := phone.signIn(t, f.mail, anaEmail); err != nil {
		t.Errorf("Ana's phone: %v, want a sign-in", err)
	}
	// What is left: a browser Ana never used shares the limit per email.
	if err := f.newBrowser("192.0.2.1").requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("a new browser of Ana: err = %v, want ErrRateLimited", err)
	}
}

// Way 2: five wrong guesses used to burn the code Ana was about to type, even
// on a browser she never used before.
func TestWrongGuessesDoNotBurnTheCodeOfAnotherBrowser(t *testing.T) {
	f := newFixture(t)
	laptop := f.newBrowser(anaIP)
	if err := laptop.requestCode(anaEmail); err != nil {
		t.Fatal(err)
	}
	code := f.mail.lastCode(t, anaEmail)

	// Without a flow of its own, a guess reaches no code at all.
	stranger := f.newBrowser(attackerIP)
	for range 5 {
		if _, err := stranger.verify(anaEmail, wrongCode(code)); !errors.Is(err, auth.ErrCodeExpired) {
			t.Fatalf("a guess without a flow: err = %v, want ErrCodeExpired", err)
		}
	}
	// With one, it only reaches the attacker's own code.
	if got := f.guessWrongFrom(t, anaEmail, []string{attackerIP}, 5); got != 5 {
		t.Fatalf("the attacker made %d wrong guesses, want 5", got)
	}
	if _, err := stranger.verify(anaEmail, code); !errors.Is(err, auth.ErrCodeExpired) {
		t.Errorf("Ana's code from another browser: err = %v, want ErrCodeExpired", err)
	}

	if _, err := laptop.verify(anaEmail, code); err != nil {
		t.Errorf("Ana's code from her laptop: %v, want a sign-in", err)
	}
}

// Way 3: ten wrong guesses used to lock Ana out of codes for a day.
func TestTenWrongGuessesDoNotLockOutAKnownDevice(t *testing.T) {
	f := newFixture(t)
	phone := f.signedInBrowser(t, anaEmail, anaIP)
	if got := f.guessWrongFrom(t, anaEmail, []string{attackerIP, "198.51.100.67"}, 5); got != 10 {
		t.Fatalf("the attacker made %d wrong guesses, want 10", got)
	}

	if err := phone.signIn(t, f.mail, anaEmail); err != nil {
		t.Errorf("Ana's phone: %v, want a sign-in", err)
	}
	// What is left: a browser Ana never used is locked out with the email.
	err := f.newBrowser("192.0.2.1").requestCode(anaEmail)
	var limited *auth.RateLimitError
	if !errors.As(err, &limited) || limited.RetryAfter <= 23*time.Hour {
		t.Errorf("a new browser of Ana: err = %v, want a limit of a day", err)
	}
}

// Finding 4 stays fixed: browsers the email's User never used share 10 wrong
// guesses a day, however many flows and addresses they spread them over.
func TestUnknownBrowsersShareTenWrongGuessesADay(t *testing.T) {
	f := newFixture(t)
	f.signedInBrowser(t, anaEmail, anaIP) // Ana has an account and a phone
	var ips []string
	for i := range 40 {
		ips = append(ips, fmt.Sprintf("198.51.100.%d", i))
	}

	got := 0
	for hour := range 4 {
		got += f.guessWrongFrom(t, anaEmail, ips[hour*10:hour*10+10], 2)
		f.clock.Advance(time.Hour)
	}

	if got != 10 {
		t.Errorf("wrong guesses in 4 hours from 40 browsers = %d, want 10", got)
	}
	b := f.newBrowser("192.0.2.1")
	if err := b.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("one more browser: err = %v, want ErrRateLimited", err)
	}
}

// A known device has limits of its own: it does not get to guess forever.
func TestAKnownDeviceHasLimitsOfItsOwn(t *testing.T) {
	f := newFixture(t)
	phone := f.signedInBrowser(t, anaEmail, anaIP)
	f.clock.Advance(time.Hour)

	for i := range 5 {
		if err := phone.requestCode(anaEmail); err != nil {
			t.Fatalf("code %d: %v", i+1, err)
		}
	}
	if err := phone.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("sixth code within the hour: err = %v, want ErrRateLimited", err)
	}

	f.clock.Advance(time.Hour)
	guesses := 0
	for range 2 {
		if err := phone.requestCode(anaEmail); err != nil {
			t.Fatal(err)
		}
		code := f.mail.lastCode(t, anaEmail)
		for range 5 {
			if _, err := phone.verify(anaEmail, wrongCode(code)); errors.Is(err, auth.ErrWrongCode) {
				guesses++
			}
		}
	}
	if guesses != 10 {
		t.Fatalf("wrong guesses = %d, want 10", guesses)
	}
	if err := phone.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("after 10 wrong guesses: err = %v, want ErrRateLimited", err)
	}
	// Those were the phone's own: the email's are untouched.
	if err := f.newBrowser("192.0.2.1").signIn(t, f.mail, anaEmail); err != nil {
		t.Errorf("a new browser of Ana: %v, want a sign-in", err)
	}
}

// Even after a restart, which forgets the limits in memory, the browsers an
// email's User never used hold 5 of its codes at once at most.
func TestUnknownBrowsersHoldFewLiveCodesOfAnEmail(t *testing.T) {
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

	err = f.newBrowser("198.51.100.9").requestCodeFrom(restarted, anaEmail)

	var limited *auth.RateLimitError
	if !errors.As(err, &limited) || limited.RetryAfter != 10*time.Minute {
		t.Errorf("err = %v, want a limit until the codes expire", err)
	}
}

func TestAKnownDeviceIsOnlyKnownToItsOwnUser(t *testing.T) {
	f := newFixture(t)
	f.signedInBrowser(t, anaEmail, anaIP)
	betosLaptop := f.signedInBrowser(t, "beto@example.com", attackerIP)
	f.guessWrongFrom(t, anaEmail, []string{"198.51.100.1", "198.51.100.2"}, 5)

	if err := betosLaptop.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("Beto's laptop asking for Ana's codes: err = %v, want ErrRateLimited", err)
	}
}

// A shared laptop: each person who signed in there keeps it known.
func TestABrowserStaysKnownToEachUserWhoSignedIn(t *testing.T) {
	f := newFixture(t)
	laptop := f.signedInBrowser(t, anaEmail, anaIP)
	if err := laptop.signIn(t, f.mail, "beto@example.com"); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{anaEmail, "beto@example.com"} {
		f.guessWrongFrom(t, email, []string{"198.51.100.1", "198.51.100.2"}, 5)
	}

	for _, email := range []string{anaEmail, "beto@example.com"} {
		if err := laptop.signIn(t, f.mail, email); err != nil {
			t.Errorf("%s on the shared laptop: %v, want a sign-in", email, err)
		}
	}
	if n := strings.Count(laptop.KnownDevices, ".") + 1; n != 2 {
		t.Errorf("the cookie holds %d tokens, want one per User", n)
	}
}

// Whoever could plant a cookie of their own in Ana's browser must not learn
// the token that makes it known to her.
func TestSigningInNeverKeepsATokenOfSomeoneElse(t *testing.T) {
	f := newFixture(t)
	mallory := f.signedInBrowser(t, "mallory@example.com", attackerIP)
	planted := f.newBrowser(anaIP)
	planted.KnownDevices = mallory.KnownDevices + ".NOTATOKENOFANYBODY"

	if err := planted.signIn(t, f.mail, anaEmail); err != nil {
		t.Fatal(err)
	}

	tokens := strings.Split(planted.KnownDevices, ".")
	if len(tokens) != 2 || tokens[1] != mallory.KnownDevices || tokens[0] == mallory.KnownDevices {
		t.Fatalf("cookie = %q, want a new token for Ana and then Mallory's", planted.KnownDevices)
	}
	f.guessWrongFrom(t, anaEmail, []string{"198.51.100.1", "198.51.100.2"}, 5)
	if err := mallory.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("Mallory's browser asking for Ana's codes: err = %v, want ErrRateLimited", err)
	}
}

func TestABrowserIsForgottenAfterAYearWithoutSigningIn(t *testing.T) {
	f := newFixture(t)
	phone := f.signedInBrowser(t, anaEmail, anaIP)
	f.clock.Advance(365 * 24 * time.Hour)
	f.guessWrongFrom(t, anaEmail, []string{"198.51.100.1", "198.51.100.2"}, 5)

	if err := phone.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

// docs/datos-personales.md keeps a known device a year without use, also
// when its User never signs in again. Then only the sign-ins of other Users
// can delete it, and they look for such rows once a day.
func TestTheBrowsersOfAUserWhoNeverComesBackAreDeleted(t *testing.T) {
	f := newFixture(t)
	phone := f.newBrowser(anaIP)
	if err := phone.requestCode(anaEmail); err != nil {
		t.Fatal(err)
	}
	ana, err := phone.verify(anaEmail, f.mail.lastCode(t, anaEmail))
	if err != nil {
		t.Fatal(err)
	}

	f.clock.Advance(200 * 24 * time.Hour)
	f.signedInBrowser(t, "beto@example.com", attackerIP)
	if !strings.Contains(f.dump(t, "known_devices"), ana.User.ID) {
		t.Fatal("Ana's browser was deleted before its year")
	}

	f.clock.Advance(166 * 24 * time.Hour) // a year and a day after Ana signed in
	f.signedInBrowser(t, "carla@example.com", "192.0.2.1")
	if strings.Contains(f.dump(t, "known_devices"), ana.User.ID) {
		t.Error("known_devices still holds Ana's browser a year and a day after she last signed in")
	}
}

func TestAUserKeepsTwentyKnownDevicesAtMost(t *testing.T) {
	f := newFixture(t)
	for i := range 25 {
		f.signedInBrowser(t, anaEmail, fmt.Sprintf("198.51.100.%d", i))
		f.clock.Advance(20 * time.Minute) // keeps within the limit per email
	}

	if rows := strings.Count(f.dump(t, "known_devices"), "\n"); rows != 20 {
		t.Errorf("known devices = %d, want 20", rows)
	}
}

func TestDeletingTheAccountForgetsItsBrowsers(t *testing.T) {
	f := newFixture(t)
	phone := f.newBrowser(anaIP)
	if err := phone.requestCode(anaEmail); err != nil {
		t.Fatal(err)
	}
	ana, err := phone.verify(anaEmail, f.mail.lastCode(t, anaEmail))
	if err != nil {
		t.Fatal(err)
	}

	if err := f.svc.DeleteUser(context.Background(), ana.User.ID); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(f.dump(t, "known_devices"), ana.User.ID) {
		t.Error("known_devices still holds the deleted User")
	}
	// Whoever signs up with the email next is someone the phone does not know.
	f.guessWrongFrom(t, anaEmail, []string{"198.51.100.1", "198.51.100.2"}, 5)
	if err := phone.requestCode(anaEmail); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}
