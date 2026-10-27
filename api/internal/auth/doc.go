// Package auth signs Users in with a 6-digit login code sent by email or with
// Google OpenID Connect (ADR 0005), and manages sessions: an HttpOnly cookie
// with an opaque token whose hash is the only thing stored.
package auth
