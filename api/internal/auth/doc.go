// Package auth signs Users in with a 6-digit login code sent by email or with
// Google OpenID Connect (ADR 0005), and manages sessions: an HttpOnly cookie
// with an opaque token whose hash is the only thing stored. A code only works
// in the browser that asked for it, and the browsers a User signed in from
// have limits of their own, so that nobody can lock that User out (ADR 0011).
// It also deletes accounts, which undoes all of that.
package auth
