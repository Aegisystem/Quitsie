package httpapi

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"

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
)

// limitBody stops reading a request at limit bytes. Decoding the body then
// fails with an *http.MaxBytesError, which badRequest answers.
func limitBody(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// requestLimit returns the middleware that answers 429 to a client over its
// requestsPerMinute, before the request costs anything else. Everything it
// wraps counts against the same limit.
func (s *Server) requestLimit() func(http.Handler) http.Handler {
	now := s.deps.Now
	if now == nil {
		now = time.Now
	}
	limiter := ratelimit.New(requestsPerMinute, maxTrackedClients, now)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := limiter.Allow(clientIP(r))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
				writeProblem(w, http.StatusTooManyRequests, ProblemCodeRateLimited, "Too many requests.", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP is who a request counts as for the limits per IP.
//
// Whether a request came through the tunnel is decided by the peer of its
// connection, which a client cannot forge. cloudflared runs on this machine
// and connects over loopback, so that peer means "behind the tunnel", where
// every visitor shares 127.0.0.1 and Cloudflare writes the real address in
// CF-Connecting-IP. Any other peer reached this port directly, which only
// happens if QUITSIE_ADDR leaves loopback: nothing stopped it from writing that
// header, so only its own address counts. On the VM the port is not public,
// and cloudflared must stay the only process that forwards traffic to it.
func clientIP(r *http.Request) string {
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
	// One IPv6 customer holds a whole /64, so a single address means nothing.
	if ip.Is6() {
		if network, err := ip.Prefix(64); err == nil {
			return network.String()
		}
	}
	return ip.String()
}
