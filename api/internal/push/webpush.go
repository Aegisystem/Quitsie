package push

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Keys are the VAPID key pair that signs the notifications (RFC 8292), in
// unpadded base64url, as browsers take them. The PWA subscribes with Public;
// a device subscribed with another key stops getting notifications until it
// subscribes again.
type Keys struct {
	Public  string
	Private string
}

// GenerateKeys makes a new key pair, for `quitsie vapid-keys`.
func GenerateKeys() (Keys, error) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return Keys{}, err
	}
	return Keys{
		Public:  base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Private: base64.RawURLEncoding.EncodeToString(key.Bytes()),
	}, nil
}

// Check says whether k is a P-256 key pair whose halves belong together.
func (k Keys) Check() error {
	private, err := base64.RawURLEncoding.DecodeString(k.Private)
	if err != nil {
		return errors.New("the private key is not unpadded base64url")
	}
	key, err := ecdh.P256().NewPrivateKey(private)
	if err != nil {
		return errors.New("the private key is not a P-256 key")
	}
	public, err := base64.RawURLEncoding.DecodeString(k.Public)
	if err != nil {
		return errors.New("the public key is not unpadded base64url")
	}
	if !bytes.Equal(public, key.PublicKey().Bytes()) {
		return errors.New("the public key does not belong to the private key")
	}
	return nil
}

// services are the hosts of the push services of the browsers with Web Push.
// The server only sends to them: an endpoint comes from the client, and any
// other host would let a client make the server post wherever it says
// (SSRF). An entry that starts with a dot takes any subdomain.
var services = []string{
	"fcm.googleapis.com",                // Chrome, Android and other Chromium browsers
	"android.googleapis.com",            // the same, older subscriptions
	"updates.push.services.mozilla.com", // Firefox
	"web.push.apple.com",                // Safari, and PWAs on iOS
	".push.apple.com",
	".notify.windows.com", // Edge
}

// ErrUnknownService is what CheckEndpoint returns for an endpoint outside
// the push services.
var ErrUnknownService = errors.New("the endpoint is not of a known push service")

// maxEndpoint is far above the longest endpoints browsers give, of about 500
// characters.
const maxEndpoint = 2048

// CheckEndpoint says whether endpoint is an HTTPS URL of a push service.
func CheckEndpoint(endpoint string) error {
	if len(endpoint) > maxEndpoint {
		return ErrUnknownService
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return ErrUnknownService
	}
	host := strings.ToLower(u.Hostname())
	for _, s := range services {
		if host == s || (strings.HasPrefix(s, ".") && strings.HasSuffix(host, s)) {
			return nil
		}
	}
	return ErrUnknownService
}

// CheckDeviceKeys says whether p256dh and auth are the keys a browser gives:
// a P-256 public key and a secret of 16 bytes, in base64url.
func CheckDeviceKeys(p256dh, auth string) error {
	public, err := decodeKey(p256dh)
	if err != nil {
		return errors.New("p256dh is not base64url")
	}
	if _, err := ecdh.P256().NewPublicKey(public); err != nil {
		return errors.New("p256dh is not a P-256 public key")
	}
	secret, err := decodeKey(auth)
	if err != nil || len(secret) != 16 {
		return errors.New("auth is not a secret of 16 bytes in base64url")
	}
	return nil
}

// decodeKey reads base64url with or without padding, as browsers differ.
func decodeKey(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// TTL is how long a push service keeps a notification for a device that is
// off. The PWA shows the records anyway when it opens.
const TTL = 12 * time.Hour

// WebPush sends notifications through the browsers' push services, encrypted
// for each device (RFC 8291) and signed with the VAPID keys.
type WebPush struct {
	keys Keys
	// subject is how a push service reaches the owner: the app's public
	// origin.
	subject string
	client  *http.Client
	// check is CheckEndpoint; the tests send to a local server.
	check func(string) error
}

func NewWebPush(keys Keys, subject string) *WebPush {
	return &WebPush{keys: keys, subject: subject, client: &http.Client{Timeout: 10 * time.Second}, check: CheckEndpoint}
}

// ServiceError is a push service that answered with an error.
type ServiceError struct{ Status int }

func (e *ServiceError) Error() string { return fmt.Sprintf("the push service answered %d", e.Status) }

func (w *WebPush) Send(ctx context.Context, to Subscription, payload []byte) error {
	if err := w.check(to.Endpoint); err != nil {
		return err
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: to.Endpoint,
		Keys:     webpush.Keys{P256dh: to.P256dh, Auth: to.Auth},
	}, &webpush.Options{
		HTTPClient:      w.client,
		Subscriber:      w.subject,
		TTL:             int(TTL.Seconds()),
		Urgency:         webpush.UrgencyNormal,
		VAPIDPublicKey:  w.keys.Public,
		VAPIDPrivateKey: w.keys.Private,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode >= 300:
		return &ServiceError{Status: resp.StatusCode}
	}
	return nil
}
