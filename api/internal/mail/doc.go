// Package mail sends emails, such as login codes, through Resend.
package mail

import "context"

// Message is a plain-text email to a single recipient.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender delivers emails. Resend sends them for real; LogSender writes them
// to the log for development.
type Sender interface {
	Send(ctx context.Context, m Message) error
}
