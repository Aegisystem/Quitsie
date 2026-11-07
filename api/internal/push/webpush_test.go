package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneratedKeysBelongTogether(t *testing.T) {
	keys, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Check(); err != nil {
		t.Fatal(err)
	}
	other, _ := GenerateKeys()
	if err := (Keys{Public: other.Public, Private: keys.Private}).Check(); err == nil {
		t.Error("a public key of another pair passed the check")
	}
	if err := (Keys{Public: keys.Public, Private: "not a key"}).Check(); err == nil {
		t.Error("a broken private key passed the check")
	}
}

func TestOnlyThePushServicesOfBrowsersAreEndpoints(t *testing.T) {
	for endpoint, ok := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":                 true,
		"https://updates.push.services.mozilla.com/wpush/v2/abc":  true,
		"https://web.push.apple.com/QGx1":                         true,
		"https://api.push.apple.com/3/device/abc":                 true,
		"https://wns2-par02p.notify.windows.com/w/?token=abc":     true,
		"http://fcm.googleapis.com/fcm/send/abc":                  false,
		"https://fcm.googleapis.com:8443/fcm/send/abc":            false,
		"https://user@fcm.googleapis.com/fcm/send/abc":            false,
		"https://fcm.googleapis.com.example.com/abc":              false,
		"https://evilpush.apple.com.example.com/":                 false,
		"https://localhost/api/v1/me":                             false,
		"https://169.254.169.254/latest/meta-data":                false,
		"https://fcm.googleapis.com/" + strings.Repeat("a", 2100): false,
		"not a url": false,
	} {
		if err := CheckEndpoint(endpoint); (err == nil) != ok {
			t.Errorf("CheckEndpoint(%.60q) = %v, want ok=%v", endpoint, err, ok)
		}
	}
}

func TestTheKeysOfADevice(t *testing.T) {
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	p256dh := base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	secret := make([]byte, 16)
	auth := base64.RawURLEncoding.EncodeToString(secret)
	if err := CheckDeviceKeys(p256dh, auth); err != nil {
		t.Fatal(err)
	}
	// Some browsers pad them.
	if err := CheckDeviceKeys(p256dh+"=", auth+"=="); err != nil {
		t.Errorf("padded keys: %v", err)
	}
	if err := CheckDeviceKeys("AAAA", auth); err == nil {
		t.Error("a short p256dh passed")
	}
	if err := CheckDeviceKeys(p256dh, "AAAA"); err == nil {
		t.Error("a short auth passed")
	}
}

func TestWhatThePushServiceAnswers(t *testing.T) {
	keys, _ := GenerateKeys()
	device, _ := ecdh.P256().GenerateKey(rand.Reader)
	to := Subscription{
		P256dh: base64.RawURLEncoding.EncodeToString(device.PublicKey().Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
	}
	for status, want := range map[int]error{
		http.StatusCreated:         nil,
		http.StatusGone:            ErrGone,
		http.StatusNotFound:        ErrGone,
		http.StatusTooManyRequests: &ServiceError{Status: http.StatusTooManyRequests},
	} {
		var got *http.Request
		service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r
			w.WriteHeader(status)
		}))
		w := NewWebPush(keys, "https://quitsie.example")
		w.check = func(string) error { return nil }
		to.Endpoint = service.URL + "/push/abc"
		err := w.Send(context.Background(), to, []byte(`{"title":"Viaje"}`))
		service.Close()

		var serviceErr *ServiceError
		switch {
		case want == nil && err != nil, want == ErrGone && !errors.Is(err, ErrGone):
			t.Errorf("%d: err = %v, want %v", status, err, want)
		case status == http.StatusTooManyRequests && (!errors.As(err, &serviceErr) || serviceErr.Status != status):
			t.Errorf("%d: err = %v, want a ServiceError", status, err)
		}
		if got.Header.Get("Content-Encoding") != "aes128gcm" || got.Header.Get("TTL") != "43200" || got.Header.Get("Urgency") != "normal" {
			t.Errorf("headers = %v", got.Header)
		}
		if a := got.Header.Get("Authorization"); !strings.HasPrefix(a, "vapid t=") || !strings.HasSuffix(a, "k="+keys.Public) {
			t.Errorf("Authorization = %q", a)
		}
	}
}

func TestTheSenderRefusesAnUnknownService(t *testing.T) {
	keys, _ := GenerateKeys()
	err := NewWebPush(keys, "https://quitsie.example").Send(context.Background(), Subscription{Endpoint: "https://127.0.0.1/"}, nil)
	if !errors.Is(err, ErrUnknownService) {
		t.Errorf("err = %v, want ErrUnknownService", err)
	}
}
