package main

import (
	"bytes"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/mail"
)

// production is a complete production environment.
var production = map[string]string{
	"QUITSIE_PUBLIC_ORIGIN":  "https://quitsie.app",
	"QUITSIE_RESEND_API_KEY": "re_test_key",
	"QUITSIE_MAIL_FROM":      "Quitsie <codigos@quitsie.app>",
	"QUITSIE_INVITE_SECRET":  strings.Repeat("s", 32),
}

// development is what a developer sets to run the server locally.
var development = map[string]string{
	"QUITSIE_ENV":  "development",
	"QUITSIE_MAIL": "log",
}

// env returns a getenv for base with changes on top; an empty value unsets.
func env(base, changes map[string]string) func(string) string {
	vars := maps.Clone(base)
	maps.Copy(vars, changes)
	return func(key string) string { return vars[key] }
}

func TestAProductionEnvironment(t *testing.T) {
	cfg, err := loadConfig(env(production, map[string]string{"QUITSIE_DEFAULT_CURRENCY": "COP"}))
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(cfg.origins, []string{"https://quitsie.app"}) {
		t.Errorf("origins = %v, want only the public one", cfg.origins)
	}
	if string(cfg.inviteSecret) != production["QUITSIE_INVITE_SECRET"] || cfg.inviteSecretGenerated {
		t.Errorf("invite secret = %q (generated %v), want the configured one", cfg.inviteSecret, cfg.inviteSecretGenerated)
	}
	if cfg.defaultCurrency != "COP" {
		t.Errorf("default currency = %q, want COP", cfg.defaultCurrency)
	}
	if cfg.google != nil {
		t.Errorf("google = %+v, want it off when it has no client", cfg.google)
	}
	sender, err := cfg.mailSender(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sender.(*mail.Resend); !ok {
		t.Errorf("mail sender = %T, want Resend", sender)
	}
}

func TestTheDefaultCurrencyIsDollars(t *testing.T) {
	cfg, err := loadConfig(env(production, nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.defaultCurrency != "USD" {
		t.Errorf("default currency = %q, want USD", cfg.defaultCurrency)
	}
}

func TestGoogleUsesThePublicOrigin(t *testing.T) {
	cfg, err := loadConfig(env(production, map[string]string{
		"QUITSIE_GOOGLE_CLIENT_ID":     "id.apps.googleusercontent.com",
		"QUITSIE_GOOGLE_CLIENT_SECRET": "secret",
	}))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.google == nil || cfg.google.ClientID != "id.apps.googleusercontent.com" || cfg.google.ClientSecret != "secret" {
		t.Fatalf("google = %+v, want the configured client", cfg.google)
	}
	if want := "https://quitsie.app/api/v1/auth/google/callback"; cfg.google.RedirectURL != want {
		t.Errorf("RedirectURL = %q, want %q", cfg.google.RedirectURL, want)
	}
}

func TestProductionRefusesToStartWithABadEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		changes map[string]string
		// mention is the variable the error must name.
		mention string
	}{
		{"no public origin", map[string]string{"QUITSIE_PUBLIC_ORIGIN": ""}, "QUITSIE_PUBLIC_ORIGIN"},
		{"plain HTTP origin", map[string]string{"QUITSIE_PUBLIC_ORIGIN": "http://quitsie.app"}, "QUITSIE_PUBLIC_ORIGIN"},
		{"origin with a path", map[string]string{"QUITSIE_PUBLIC_ORIGIN": "https://quitsie.app/app"}, "QUITSIE_PUBLIC_ORIGIN"},
		{"origin with a trailing slash", map[string]string{"QUITSIE_PUBLIC_ORIGIN": "https://quitsie.app/"}, "QUITSIE_PUBLIC_ORIGIN"},
		{"no Resend key", map[string]string{"QUITSIE_RESEND_API_KEY": ""}, "QUITSIE_RESEND_API_KEY"},
		{"no sender", map[string]string{"QUITSIE_MAIL_FROM": ""}, "QUITSIE_MAIL_FROM"},
		{"no invite secret", map[string]string{"QUITSIE_INVITE_SECRET": ""}, "QUITSIE_INVITE_SECRET"},
		{"short invite secret", map[string]string{"QUITSIE_INVITE_SECRET": "short"}, "QUITSIE_INVITE_SECRET"},
		{"Google without a secret", map[string]string{"QUITSIE_GOOGLE_CLIENT_ID": "id"}, "QUITSIE_GOOGLE_CLIENT_SECRET"},
		{"Google without a client", map[string]string{"QUITSIE_GOOGLE_CLIENT_SECRET": "secret"}, "QUITSIE_GOOGLE_CLIENT_ID"},
		{"unknown currency", map[string]string{"QUITSIE_DEFAULT_CURRENCY": "XYZ"}, "QUITSIE_DEFAULT_CURRENCY"},
		{"unknown environment", map[string]string{"QUITSIE_ENV": "staging"}, "QUITSIE_ENV"},
		{"unknown mail sender", map[string]string{"QUITSIE_MAIL": "smtp"}, "QUITSIE_MAIL"},
		// Anyone who reads the log could sign in as anyone.
		{"development mail", map[string]string{"QUITSIE_MAIL": "log"}, "QUITSIE_MAIL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(env(production, tt.changes))

			if err == nil || !strings.Contains(err.Error(), tt.mention) {
				t.Errorf("err = %v, want one that names %s", err, tt.mention)
			}
		})
	}
}

// A missing key must stop the server, even in development, never send the
// login codes to the log.
func TestAMissingKeyNeverTurnsOnTheDevelopmentMail(t *testing.T) {
	_, err := loadConfig(env(map[string]string{"QUITSIE_ENV": "development"}, nil))

	if err == nil || !strings.Contains(err.Error(), "QUITSIE_RESEND_API_KEY") {
		t.Errorf("err = %v, want one that names QUITSIE_RESEND_API_KEY", err)
	}
}

// It is the first thing someone sees who runs the server with no setup.
func TestAnEmptyEnvironmentSaysHowToRunLocally(t *testing.T) {
	_, err := loadConfig(env(map[string]string{}, nil))

	if err == nil || !strings.Contains(err.Error(), "QUITSIE_ENV=development") {
		t.Errorf("err = %v, want one that mentions QUITSIE_ENV=development", err)
	}
}

func TestADevelopmentEnvironment(t *testing.T) {
	cfg, err := loadConfig(env(development, nil))
	if err != nil {
		t.Fatal(err)
	}

	// Vite serves the PWA on its own port and proxies /api to this server.
	if !slices.Equal(cfg.origins, []string{"http://localhost:8080", "http://localhost:5173"}) {
		t.Errorf("origins = %v, want this server and Vite", cfg.origins)
	}
	if len(cfg.inviteSecret) != 32 || !cfg.inviteSecretGenerated {
		t.Errorf("invite secret of %d bytes (generated %v), want 32 generated ones", len(cfg.inviteSecret), cfg.inviteSecretGenerated)
	}
	again, err := loadConfig(env(development, nil))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cfg.inviteSecret, again.inviteSecret) {
		t.Error("two generated invite secrets are equal, want random ones")
	}
	sender, err := cfg.mailSender(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sender.(*mail.LogSender); !ok {
		t.Errorf("mail sender = %T, want the development one", sender)
	}
}

func TestDevelopmentKeepsAConfiguredInviteSecret(t *testing.T) {
	secret := strings.Repeat("d", 40)
	cfg, err := loadConfig(env(development, map[string]string{"QUITSIE_INVITE_SECRET": secret}))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg.inviteSecret) != secret || cfg.inviteSecretGenerated {
		t.Errorf("invite secret = %q (generated %v), want the configured one", cfg.inviteSecret, cfg.inviteSecretGenerated)
	}
}

func TestDevelopmentCanPointAtAnotherOrigin(t *testing.T) {
	cfg, err := loadConfig(env(development, map[string]string{"QUITSIE_PUBLIC_ORIGIN": "http://localhost:5173"}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.origins, []string{"http://localhost:5173"}) {
		t.Errorf("origins = %v, want Vite once", cfg.origins)
	}
}
