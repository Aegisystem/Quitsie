package httpapi

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/ratelimit"
)

const (
	// maxBodyBytes is the largest request body the API reads. POST /records
	// sets it: 100 Expenses and 100 Settlements in a Group of 50 Members, all
	// of them paying and owing, take about 1 MiB.
	maxBodyBytes = 2 << 20

	// maxAuthBodyBytes is the largest body of POST /auth/*. Those requests carry
	// an email and a code from someone unknown, so a parser never gets more.
	maxAuthBodyBytes = 4 << 10

	// requestsPerMinute is what one IP gets, on top of Cloudflare's own limit.
	// It is generous because a whole carrier's NAT can share one address, and
	// the PWA makes a few requests per minute at most.
	requestsPerMinute = 300

	// maxTrackedClients bounds the memory of that limit to some 15 MB. Reaching
	// it takes that many addresses within one minute.
	maxTrackedClients = 100_000

	// writeKiBPerHour is how much a User can send in changes. It lets through
	// five of the largest batches at once and a steady trickle after that, and
	// it is what keeps one account from filling the disk.
	writeKiBPerHour = 5 << 10

	// maxPullsAtOnce is how many GET /sync run at a time. Each can take around
	// 100 MB until its answer is out, and the VM has 1 GB. A pull that waits
	// longer than pullWait answers 429, and the client tries again.
	maxPullsAtOnce = 2
	pullWait       = 5 * time.Second
)

// limitBody stops reading a request at limit bytes. Decoding the body then
// fails with an *http.MaxBytesError, which badRequest answers.
func limitBody(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// gate lets n requests through next at a time. A request that cannot get in
// within wait answers 429, and one that gives up leaves the queue. It wraps the
// handler and not the operation because writing the answer is what costs most.
func gate(n int, wait time.Duration, next http.Handler) http.Handler {
	slots := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout := time.NewTimer(wait)
		defer timeout.Stop()
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		case <-timeout.C:
			tooMany(w, time.Second, "The server is busy.")
		case <-r.Context().Done():
		}
	})
}

// onePerKey answers 429 to a request whose key already has one running. Requests
// without a key go through.
func onePerKey(key func(*http.Request) (string, bool), next http.Handler) http.Handler {
	var running sync.Map
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k, ok := key(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if _, taken := running.LoadOrStore(k, struct{}{}); taken {
			tooMany(w, time.Second, "One at a time.")
			return
		}
		defer running.Delete(k)
		next.ServeHTTP(w, r)
	})
}

// pullingUser is who a GET /sync belongs to, so that one account never takes more
// than one of the places of the gate. It needs Authenticate to have run.
func pullingUser(r *http.Request) (string, bool) {
	user, signedIn := auth.CurrentUser(r.Context())
	return user.ID, signedIn && r.Method == http.MethodGet && r.URL.Path == "/api/v1/sync"
}

// requestLimit returns the middleware that answers 429 to a client over its
// requestsPerMinute, before the request costs anything else. Everything it
// wraps counts against the same limit.
func (s *Server) requestLimit() func(http.Handler) http.Handler {
	limiter := ratelimit.New(requestsPerMinute, time.Minute, maxTrackedClients, s.clock())
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ok, retryAfter := limiter.Allow(clientIP(r)); !ok {
				tooMany(w, retryAfter, "Too many requests.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeBudget answers 429 to a User who sent writeKiBPerHour in changes. It only
// charges what gets stored: not reads, not whoever has no session, and not
// /auth/, so that signing out or closing a stolen session always works. It needs
// Authenticate to have run.
func (s *Server) writeBudget(next http.Handler) http.Handler {
	limiter := ratelimit.New(writeKiBPerHour, time.Hour, maxTrackedClients, s.clock())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, signedIn := auth.CurrentUser(r.Context())
		if !signedIn || r.Method == http.MethodGet || r.Method == http.MethodHead || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		if ok, retryAfter := limiter.AllowN(user.ID, costInKiB(r)); !ok {
			tooMany(w, retryAfter, "Too many changes.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// costInKiB is what a request spends of writeKiBPerHour. Even an empty one
// leaves a row behind, and a body of unknown length may be the largest there is.
func costInKiB(r *http.Request) int {
	size := r.ContentLength
	if size < 0 {
		size = maxBodyBytes
	}
	return 1 + int(size>>10)
}

func tooMany(w http.ResponseWriter, retryAfter time.Duration, title string) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	writeProblem(w, http.StatusTooManyRequests, ProblemCodeRateLimited, title, "")
}

// clock is the time source of the limits.
func (s *Server) clock() func() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now
	}
	return time.Now
}

// IPv6 customers get a whole network, not one address: a /64 at least, and
// often a /56 that holds 256 of them. The limits count each network as one
// client: requests by the /64, and login codes by the /56, because each one
// is an email out of the server's budget.
const (
	requestPrefixBits = 64
	sendPrefixBits    = 56
)

// clientIP is who a request counts as for the limits per IP.
func clientIP(r *http.Request) string {
	return clientNetwork(r, requestPrefixBits)
}

// senderIP is who a request for a login code counts as for the limits on
// sending codes.
func senderIP(r *http.Request) string {
	return clientNetwork(r, sendPrefixBits)
}

// clientNetwork is the address a request comes from, or its network of
// ipv6Bits in IPv6.
//
// Whether a request came through the tunnel is decided by the peer of its
// connection, which a client cannot forge. cloudflared runs on this machine
// and connects over loopback, so that peer means "behind the tunnel", where
// every visitor shares 127.0.0.1 and Cloudflare writes the real address in
// CF-Connecting-IP. Any other peer reached this port directly, which only
// happens if QUITSIE_ADDR leaves loopback: nothing stopped it from writing that
// header, so only its own address counts. On the VM the port is not public,
// and cloudflared must stay the only process that forwards traffic to it.
func clientNetwork(r *http.Request, ipv6Bits int) string {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	ip := peer.Addr().Unmap()
	if ip.IsLoopback() {
		if visitor, err := netip.ParseAddr(r.Header.Get("CF-Connecting-IP")); err == nil {
			ip = visitor.Unmap()
		}
	}
	if ip.Is6() {
		if network, err := ip.Prefix(ipv6Bits); err == nil {
			return network.String()
		}
	}
	return ip.String()
}
