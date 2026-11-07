package httpapi_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/internal/storetest"
)

const vapidPublicKey = "BPublicKeyOfTheServer"

// pushAPI is recordsAPI on a server that sends push notifications. A User
// can sign in again at once, to open a session on a second device.
func pushAPI(t *testing.T) *recordsAPI {
	t.Helper()
	db := storetest.Open(t)
	box := &codeBox{}
	svc, err := auth.New(auth.Config{DB: db, Mail: box, DefaultCurrency: "USD", SendCooldown: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{DB: db, Auth: svc, Origins: []string{testOrigin}, PushKey: vapidPublicKey})
	return &recordsAPI{t: t, db: db, auth: svc, codeBox: box, handler: httpapi.NewHandler(server, builtPWA)}
}

// device is what a browser's PushSubscription.toJSON() gives.
func device(endpoint string) httpapi.PushSubscription {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	var d httpapi.PushSubscription
	d.Endpoint = endpoint
	d.Keys.P256dh = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	d.Keys.Auth = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	return d
}

// endpointsOf are the endpoints that notify p's User.
func (a *recordsAPI) endpointsOf(p signedIn) []string {
	a.t.Helper()
	subs, err := store.New(a.db).ListPushSubscriptionsOfUser(context.Background(), store.ListPushSubscriptionsOfUserParams{UserID: p.user.ID, UsedAfter: time.Now().Add(-time.Hour)})
	if err != nil {
		a.t.Fatal(err)
	}
	var endpoints []string
	for _, s := range subs {
		endpoints = append(endpoints, s.Endpoint)
	}
	return endpoints
}

func TestTheKeyOfPushNotifications(t *testing.T) {
	a := pushAPI(t)
	ana := a.signIn("ana@example.com")
	if got := decodeBody[httpapi.PushKey](t, a.do(ana, http.MethodGet, "/api/v1/push/key", nil), http.StatusOK); got.PublicKey != vapidPublicKey {
		t.Errorf("publicKey = %q", got.PublicKey)
	}

	// A server without VAPID keys sends no notifications.
	off := newRecordsAPI(t)
	beto := off.signIn("beto@example.com")
	decodeProblem(t, off.do(beto, http.MethodGet, "/api/v1/push/key", nil), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decodeProblem(t, off.do(beto, http.MethodPut, "/api/v1/push/subscription", device("https://fcm.googleapis.com/fcm/send/a")), http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

func TestADeviceTurnsNotificationsOnAndOff(t *testing.T) {
	a := pushAPI(t)
	ana := a.signIn("ana@example.com")
	first, second := "https://web.push.apple.com/first", "https://web.push.apple.com/second"

	if rec := a.do(ana, http.MethodPut, "/api/v1/push/subscription", device(first)); rec.Code != http.StatusNoContent {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	// Saving it again changes nothing.
	a.do(ana, http.MethodPut, "/api/v1/push/subscription", device(first))
	if got := a.endpointsOf(ana); len(got) != 1 || got[0] != first {
		t.Fatalf("endpoints = %v, want the first", got)
	}
	// The browser subscribed again: the new endpoint replaces the old one.
	a.do(ana, http.MethodPut, "/api/v1/push/subscription", device(second))
	if got := a.endpointsOf(ana); len(got) != 1 || got[0] != second {
		t.Fatalf("endpoints = %v, want the second", got)
	}

	if rec := a.do(ana, http.MethodDelete, "/api/v1/push/subscription", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body)
	}
	if got := a.endpointsOf(ana); len(got) != 0 {
		t.Errorf("endpoints = %v after turning them off", got)
	}
}

func TestEachDeviceOfAUserHasItsOwnSubscription(t *testing.T) {
	a := pushAPI(t)
	phone := a.signIn("ana@example.com")
	laptop := a.signIn("ana@example.com")
	a.do(phone, http.MethodPut, "/api/v1/push/subscription", device("https://web.push.apple.com/phone"))
	a.do(laptop, http.MethodPut, "/api/v1/push/subscription", device("https://fcm.googleapis.com/fcm/send/laptop"))
	if got := a.endpointsOf(phone); len(got) != 2 {
		t.Fatalf("endpoints = %v, want both devices", got)
	}

	// Signing out on the laptop turns off its notifications only.
	if rec := a.do(laptop, http.MethodPost, "/api/v1/auth/logout", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}
	if got := a.endpointsOf(phone); len(got) != 1 || got[0] != "https://web.push.apple.com/phone" {
		t.Errorf("endpoints = %v, want only the phone", got)
	}
}

func TestABrowserNotifiesOnlyItsLastUser(t *testing.T) {
	a := pushAPI(t)
	ana, beto := a.signIn("ana@example.com"), a.signIn("beto@example.com")
	shared := "https://fcm.googleapis.com/fcm/send/shared"
	a.do(ana, http.MethodPut, "/api/v1/push/subscription", device(shared))
	a.do(beto, http.MethodPut, "/api/v1/push/subscription", device(shared))
	if got := a.endpointsOf(ana); len(got) != 0 {
		t.Errorf("Ana still gets notifications on %v", got)
	}
	if got := a.endpointsOf(beto); len(got) != 1 {
		t.Errorf("Beto's endpoints = %v", got)
	}
}

func TestOnlyThePushServiceOfABrowserIsAnEndpoint(t *testing.T) {
	a := pushAPI(t)
	ana := a.signIn("ana@example.com")
	for _, endpoint := range []string{"https://169.254.169.254/latest/meta-data", "http://fcm.googleapis.com/fcm/send/a", "https://quitsie.test/api/v1/me"} {
		decodeProblem(t, a.do(ana, http.MethodPut, "/api/v1/push/subscription", device(endpoint)), http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
	}
	broken := device("https://fcm.googleapis.com/fcm/send/a")
	broken.Keys.P256dh = "AAAA"
	decodeProblem(t, a.do(ana, http.MethodPut, "/api/v1/push/subscription", broken), http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
	if got := a.endpointsOf(ana); len(got) != 0 {
		t.Errorf("endpoints = %v", got)
	}
}
