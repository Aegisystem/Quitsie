package auth

import (
	"net/http"
	"reflect"
	"testing"
)

// A browser takes a deletion only for the cookie with its name, path and
// domain, and the prefix __Host- holds it to the same rules as the cookie.
func TestEachDeletionHasTheAttributesOfItsCookie(t *testing.T) {
	for _, pair := range []struct{ cookie, deletion *http.Cookie }{
		{SessionCookie("token"), ClearSessionCookie()},
		{codeFlowCookie("flow"), ClearCodeFlowCookie()},
		{hostCookie(GoogleCookieName, "flow", googleFlowLifetime), ClearGoogleCookie()},
	} {
		deletion := *pair.deletion
		if deletion.MaxAge >= 0 || deletion.Value != "" {
			t.Errorf("deletion = %q, want an empty value and a negative MaxAge", deletion.String())
		}
		cookie := *pair.cookie
		cookie.Value, cookie.MaxAge = deletion.Value, deletion.MaxAge
		if !reflect.DeepEqual(cookie, deletion) {
			t.Errorf("deletion = %q, want the attributes of %q", deletion.String(), pair.cookie.String())
		}
	}
}
