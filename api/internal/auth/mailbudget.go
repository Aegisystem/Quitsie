package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// The email provider sends a fixed number of emails for free (Resend: 100 a
// day and 3,000 a month), and past that the codes stop arriving for everyone
// and bounces put the account at risk. So the server keeps a budget of its own
// below that, in the database, because every deploy restarts it. The figures
// are provisional: Config can change them until real traffic says what is
// normal.
const (
	DefaultMailPerHour = 40
	DefaultMailPerDay  = 90
	mailBudgetDay      = 24 * time.Hour
)

// ErrMailBudgetSpent means the server already sent every email its budget
// allows in this clock hour or in the last 24 hours, so no code goes out to
// anyone. RequestCode returns it as a *MailBudgetError.
var ErrMailBudgetSpent = errors.New("auth: the budget of emails is spent")

// MailBudgetError is ErrMailBudgetSpent with how long until the server can
// send another email.
type MailBudgetError struct {
	RetryAfter time.Duration
}

func (e *MailBudgetError) Error() string        { return ErrMailBudgetSpent.Error() }
func (e *MailBudgetError) Is(target error) bool { return target == ErrMailBudgetSpent }

// mailBudget is what the server sent in the current clock hour and in the
// 24 hours that end with it.
type mailBudget struct {
	hour    time.Time
	perHour int
	perDay  int
	// sends are the hours of those 24 with an email, oldest first.
	sends []store.MailSend
}

func (s *Service) mailBudget(ctx context.Context, q *store.Queries, now time.Time) (mailBudget, error) {
	b := mailBudget{hour: now.Truncate(time.Hour), perHour: s.mailPerHour, perDay: s.mailPerDay}
	sends, err := q.ListMailSends(ctx, b.hour.Add(-mailBudgetDay))
	if err != nil {
		return mailBudget{}, fmt.Errorf("auth: read the budget of emails: %w", err)
	}
	b.sends = sends
	return b, nil
}

func (b mailBudget) sentThisHour() int {
	if n := len(b.sends); n > 0 && b.sends[n-1].Hour.Equal(b.hour) {
		return int(b.sends[n-1].Sent)
	}
	return 0
}

func (b mailBudget) sentToday() int {
	sent := 0
	for _, h := range b.sends {
		sent += int(h.Sent)
	}
	return sent
}

// wait is how long until one more email fits in the hour and in the day: zero
// if it fits now. The day makes room as its oldest hours leave it.
func (b mailBudget) wait(now time.Time) time.Duration {
	var wait time.Duration
	if b.sentThisHour() >= b.perHour {
		wait = b.hour.Add(time.Hour).Sub(now)
	}
	over := b.sentToday() - b.perDay + 1
	for _, h := range b.sends {
		if over <= 0 {
			break
		}
		if over -= int(h.Sent); over <= 0 {
			wait = max(wait, h.Hour.Add(mailBudgetDay).Sub(now))
		}
	}
	return wait
}

// checkMailBudget returns a *MailBudgetError if no email fits in the budget
// now. RequestCode asks before the limits of its sender, so that a request
// nobody could serve does not spend them.
func (s *Service) checkMailBudget(ctx context.Context) error {
	now := s.now()
	b, err := s.mailBudget(ctx, store.New(s.db), now)
	if err != nil {
		return err
	}
	if wait := b.wait(now); wait > 0 {
		return &MailBudgetError{RetryAfter: wait}
	}
	return nil
}

// spendMail counts one email against the budget, or returns a
// *MailBudgetError if it does not fit. It returns the hour it counted the
// email in, to give it back if the email cannot be sent, and whether that
// email spends the last of the hour or the day.
func (s *Service) spendMail(ctx context.Context, q *store.Queries) (hour time.Time, last bool, err error) {
	now := s.now()
	b, err := s.mailBudget(ctx, q, now)
	if err != nil {
		return time.Time{}, false, err
	}
	if wait := b.wait(now); wait > 0 {
		return time.Time{}, false, &MailBudgetError{RetryAfter: wait}
	}
	if err := q.DeleteOldMailSends(ctx, b.hour.Add(-mailBudgetDay)); err != nil {
		return time.Time{}, false, fmt.Errorf("auth: delete old email counts: %w", err)
	}
	if err := q.CountMailSend(ctx, b.hour); err != nil {
		return time.Time{}, false, fmt.Errorf("auth: count an email: %w", err)
	}
	return b.hour, b.sentThisHour()+1 >= b.perHour || b.sentToday()+1 >= b.perDay, nil
}

// reportMailBudgetSpent tells the owner that nobody can get a code. It runs
// once per hour or day that fills up, after the email that filled it, and not
// for each request refused afterwards. Everything that needs the owner is
// logged at ERROR level.
func (s *Service) reportMailBudgetSpent(ctx context.Context) {
	now := s.now()
	b, err := s.mailBudget(ctx, store.New(s.db), now)
	if err != nil {
		s.logger.ErrorContext(ctx, "the email budget is spent: no more login codes for now", "err", err)
		return
	}
	s.logger.ErrorContext(ctx, "the email budget is spent: no more login codes until resumes_at",
		"sent_this_hour", b.sentThisHour(), "max_per_hour", b.perHour,
		"sent_last_24h", b.sentToday(), "max_per_day", b.perDay,
		"resumes_at", now.Add(b.wait(now)).Format(time.RFC3339))
}
