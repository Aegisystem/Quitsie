package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResendConfig holds what Resend needs. APIKey and From come from the
// server's configuration, never from the code.
type ResendConfig struct {
	APIKey string
	// From is the sender, for example "Quitsie <codigos@quitsie.app>". Its
	// domain must be verified in Resend.
	From string
	// BaseURL defaults to https://api.resend.com; tests point it to a fake.
	BaseURL string
	// Client defaults to an http.Client with a 10 s timeout.
	Client *http.Client
}

// Resend sends emails with the Resend HTTP API.
type Resend struct {
	cfg ResendConfig
}

func NewResend(cfg ResendConfig) (*Resend, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("resend: missing API key")
	}
	if cfg.From == "" {
		return nil, errors.New("resend: missing sender address")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.resend.com"
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Resend{cfg: cfg}, nil
}

// resendEmail is the body of POST /emails.
type resendEmail struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (r *Resend) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(resendEmail{From: r.cfg.From, To: []string{m.To}, Subject: m.Subject, Text: m.Text})
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.BaseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.cfg.Client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("resend: %s: %s", resp.Status, detail)
	}
	return nil
}
