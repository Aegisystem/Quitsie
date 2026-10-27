package mail_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/mail"
)

func TestLogSenderWritesTheEmailToTheLog(t *testing.T) {
	var buf bytes.Buffer
	s := mail.NewLogSender(slog.New(slog.NewTextHandler(&buf, nil)))

	err := s.Send(context.Background(), mail.Message{
		To:      "ana@example.com",
		Subject: "Tu código de Quitsie: 123456",
		Text:    "Tu código es 123456.",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"ana@example.com", "Tu código es 123456."} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log %q does not contain %q", buf.String(), want)
		}
	}
}
