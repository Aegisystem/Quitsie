package mail

import (
	"context"
	"log/slog"
)

// LogSender writes emails to the log instead of sending them, so login codes
// can be read from the terminal in development. Never use it in production:
// anyone who reads the log could sign in as anyone.
type LogSender struct {
	logger *slog.Logger
}

func NewLogSender(logger *slog.Logger) *LogSender {
	return &LogSender{logger: logger}
}

func (s *LogSender) Send(ctx context.Context, m Message) error {
	s.logger.InfoContext(ctx, "email not sent (development)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}
