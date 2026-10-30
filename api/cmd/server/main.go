// Command server runs Quitsie from a single binary: the API under /api/v1,
// the embedded PWA, and the SQLite database it migrates at startup.
//
// Configuration comes from the environment. Production needs the variables
// marked as required, and the server refuses to start without them:
//
//	QUITSIE_ADDR                  address to listen on (default 127.0.0.1:8080; cloudflared connects to it)
//	QUITSIE_DB_PATH               SQLite file (default quitsie.db)
//	QUITSIE_ENV                   production (default) or development
//	QUITSIE_PUBLIC_ORIGIN         required: the app's base URL, such as https://quitsie.app (development defaults to http://localhost:8080)
//	QUITSIE_MAIL                  resend (default), or log to write emails to the log, login codes included; development only
//	QUITSIE_RESEND_API_KEY        required with resend: the key of the Resend account
//	QUITSIE_MAIL_FROM             required with resend: the sender, such as Quitsie <codigos@quitsie.app>
//	QUITSIE_GOOGLE_CLIENT_ID      OAuth client in Google Cloud; without it and its secret, Google sign-in is off
//	QUITSIE_GOOGLE_CLIENT_SECRET  secret of that client
//	QUITSIE_DEFAULT_CURRENCY      PreferredCurrency of new Users (default USD)
//	QUITSIE_INVITE_SECRET         required: at least 32 bytes that sign the Invite links; changing it revokes them all
//	                              (development generates one on each start if it is missing)
//
// To run it locally: QUITSIE_ENV=development QUITSIE_MAIL=log go run ./cmd/server
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// dist holds the PWA that `npm run build` writes from web/.
//
//go:embed all:dist
var dist embed.FS

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	sender, err := cfg.mailSender(logger)
	if err != nil {
		return err
	}
	if cfg.logMail {
		logger.Warn("development mail: emails, login codes included, go to this log")
	}
	if cfg.inviteSecretGenerated {
		logger.Warn("QUITSIE_INVITE_SECRET is not set: generated one for this run, so Invite links stop working on restart")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}

	authService, err := auth.New(auth.Config{
		DB:              db,
		Mail:            sender,
		DefaultCurrency: cfg.defaultCurrency,
		Google:          cfg.google,
	})
	if err != nil {
		return err
	}
	pwa, err := fs.Sub(dist, "dist")
	if err != nil {
		return fmt.Errorf("embedded PWA: %w", err)
	}

	api := httpapi.NewServer(db, version).WithDeps(httpapi.Deps{
		DB:           db,
		Auth:         authService,
		Origins:      cfg.origins,
		InviteSecret: cfg.inviteSecret,
	})
	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           httpapi.NewHandler(api, pwa),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	logger.Info("listening", "url", "http://"+cfg.addr, "version", version, "db", cfg.dbPath)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	// systemd sends SIGTERM on restart: finish in-flight requests, then close the database.
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// config is the server's configuration, read from the environment.
type config struct {
	addr   string
	dbPath string
	// origins are the app's own origins; requests that change data must
	// come from one of them.
	origins []string
	// logMail writes emails to the log instead of sending them with resend.
	logMail         bool
	resend          mail.ResendConfig
	google          *auth.GoogleConfig
	defaultCurrency string
	inviteSecret    []byte
	// inviteSecretGenerated says inviteSecret is a random one for this run.
	inviteSecretGenerated bool
}

const (
	googleCallbackPath = "/api/v1/auth/google/callback"
	// viteOrigin is the PWA under `npm run dev`, which proxies /api here.
	viteOrigin = "http://localhost:5173"
	// minInviteSecretLength is what invites.Token asks of its secret.
	minInviteSecretLength = 32
)

// loadConfig reads the configuration and checks it, so that a server that
// could not sign anyone in, or that would do it unsafely, never starts.
func loadConfig(getenv func(string) string) (config, error) {
	or := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
	cfg := config{
		addr:            or("QUITSIE_ADDR", "127.0.0.1:8080"),
		dbPath:          or("QUITSIE_DB_PATH", "quitsie.db"),
		defaultCurrency: or("QUITSIE_DEFAULT_CURRENCY", "USD"),
	}

	var development bool
	switch env := getenv("QUITSIE_ENV"); env {
	case "", "production":
	case "development":
		development = true
	default:
		return config{}, fmt.Errorf("QUITSIE_ENV must be production or development, not %q", env)
	}

	origin := getenv("QUITSIE_PUBLIC_ORIGIN")
	if origin == "" && development {
		origin = "http://localhost:8080"
	}
	if err := checkOrigin(origin, development); err != nil {
		return config{}, err
	}
	cfg.origins = []string{origin}
	if development && origin != viteOrigin {
		cfg.origins = append(cfg.origins, viteOrigin)
	}

	if _, ok := domain.Currency(cfg.defaultCurrency).Decimals(); !ok {
		return config{}, fmt.Errorf("QUITSIE_DEFAULT_CURRENCY is not an ISO 4217 currency: %q", cfg.defaultCurrency)
	}

	// The log mail is only ever chosen by name: a missing key stops the
	// server, because anyone who reads the log could sign in as anyone.
	switch sender := or("QUITSIE_MAIL", "resend"); sender {
	case "resend":
		cfg.resend = mail.ResendConfig{APIKey: getenv("QUITSIE_RESEND_API_KEY"), From: getenv("QUITSIE_MAIL_FROM")}
		if cfg.resend.APIKey == "" || cfg.resend.From == "" {
			return config{}, errors.New("QUITSIE_RESEND_API_KEY and QUITSIE_MAIL_FROM are required to send login codes " +
				"(to run locally, set QUITSIE_ENV=development and QUITSIE_MAIL=log)")
		}
	case "log":
		if !development {
			return config{}, errors.New("QUITSIE_MAIL=log writes login codes to the log, so it needs QUITSIE_ENV=development")
		}
		cfg.logMail = true
	default:
		return config{}, fmt.Errorf("QUITSIE_MAIL must be resend or log, not %q", sender)
	}

	id, secret := getenv("QUITSIE_GOOGLE_CLIENT_ID"), getenv("QUITSIE_GOOGLE_CLIENT_SECRET")
	switch {
	case id != "" && secret != "":
		cfg.google = &auth.GoogleConfig{ClientID: id, ClientSecret: secret, RedirectURL: origin + googleCallbackPath}
	case id != "":
		return config{}, errors.New("QUITSIE_GOOGLE_CLIENT_ID is set but QUITSIE_GOOGLE_CLIENT_SECRET is not")
	case secret != "":
		return config{}, errors.New("QUITSIE_GOOGLE_CLIENT_SECRET is set but QUITSIE_GOOGLE_CLIENT_ID is not")
	}

	cfg.inviteSecret = []byte(getenv("QUITSIE_INVITE_SECRET"))
	switch {
	case len(cfg.inviteSecret) >= minInviteSecretLength:
	case len(cfg.inviteSecret) == 0 && development:
		cfg.inviteSecret = make([]byte, minInviteSecretLength)
		rand.Read(cfg.inviteSecret)
		cfg.inviteSecretGenerated = true
	default:
		return config{}, fmt.Errorf("QUITSIE_INVITE_SECRET must have at least %d bytes", minInviteSecretLength)
	}
	return cfg, nil
}

// checkOrigin requires a bare origin, because browsers compare it character
// by character with the Origin header. Secure cookies need HTTPS, except on
// the localhost of development.
func checkOrigin(origin string, development bool) error {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || origin != u.Scheme+"://"+u.Host {
		return fmt.Errorf("QUITSIE_PUBLIC_ORIGIN must be an origin such as https://quitsie.app, not %q", origin)
	}
	if u.Scheme != "https" && !(development && u.Scheme == "http") {
		return fmt.Errorf("QUITSIE_PUBLIC_ORIGIN must use https, not %q", origin)
	}
	return nil
}

// mailSender builds the sender the configuration chose.
func (c config) mailSender(logger *slog.Logger) (mail.Sender, error) {
	if c.logMail {
		return mail.NewLogSender(logger), nil
	}
	return mail.NewResend(c.resend)
}
