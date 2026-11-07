// Command server runs Quitsie from a single binary: the API under /api/v1,
// the embedded PWA, and the PostgreSQL database it migrates at startup.
//
// Configuration comes from the environment. Production needs the variables
// marked as required, and the server refuses to start without them:
//
//	QUITSIE_ADDR                  address to listen on (default 127.0.0.1:8080, or every interface on PORT if it is set)
//	PORT                          the port a hosting platform such as Railway routes to
//	QUITSIE_DATABASE_URL          required: the PostgreSQL database, such as postgres://user:password@host:5432/quitsie
//	                              (development defaults to the one in compose.yaml)
//	QUITSIE_ENV                   production (default) or development; production also sends HSTS and
//	                              redirects visitors Cloudflare saw on plain HTTP to QUITSIE_PUBLIC_ORIGIN
//	QUITSIE_PUBLIC_ORIGIN         required: the app's base URL, such as https://quitsie.app (development defaults to http://localhost:8080)
//	QUITSIE_MAIL                  resend (default), or log to write emails to the log, login codes included; development only
//	QUITSIE_RESEND_API_KEY        required with resend: the key of the Resend account
//	QUITSIE_MAIL_FROM             required with resend: the sender, such as Quitsie <codigos@quitsie.app>
//	QUITSIE_GOOGLE_CLIENT_ID      OAuth client in Google Cloud; without it and its secret, Google sign-in is off
//	QUITSIE_GOOGLE_CLIENT_SECRET  secret of that client
//	QUITSIE_DEFAULT_CURRENCY      PreferredCurrency of new Users (default USD)
//	QUITSIE_MAIL_MAX_PER_HOUR     most login-code emails sent in a clock hour (default 40)
//	QUITSIE_MAIL_MAX_PER_DAY      most login-code emails sent in 24 hours (default 90, under Resend's free 100)
//	QUITSIE_LOGIN_CODE_COOLDOWN   development only: how long an email waits between login codes, such as 1ms
//	                              (default 1m); the end-to-end tests shorten it
//	QUITSIE_INVITE_SECRET         required: at least 32 bytes that sign the Invite links; changing it revokes them all
//	                              (development generates one on each start if it is missing)
//	QUITSIE_VAPID_PUBLIC_KEY      with QUITSIE_VAPID_PRIVATE_KEY, the key pair that signs push notifications
//	QUITSIE_VAPID_PRIVATE_KEY     (ADR 0013); `quitsie vapid-keys` prints a new one. Without them the server
//	                              sends no push notifications. Changing them turns notifications off on each
//	                              device until the PWA opens there again and subscribes with the new key
//
// To run it locally: QUITSIE_ENV=development QUITSIE_MAIL=log go run ./cmd/server
//
// After restoring a backup of the database, and before the server serves it,
// run `quitsie record-restore` once with the same environment
// (docs/despliegue.md).
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
	"strconv"
	"syscall"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/invites"
	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/push"
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
	start := run
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "record-restore":
			start = recordRestore
		case "vapid-keys":
			start = printVAPIDKeys
		}
	}
	if err := start(logger); err != nil {
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
	if cfg.vapid == nil {
		logger.Warn("QUITSIE_VAPID_PUBLIC_KEY and QUITSIE_VAPID_PRIVATE_KEY are not set: no push notifications")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.databaseURL)
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
		MailPerHour:     cfg.mailPerHour,
		MailPerDay:      cfg.mailPerDay,
		SendCooldown:    cfg.sendCooldown,
	})
	if err != nil {
		return err
	}
	pwa, err := fs.Sub(dist, "dist")
	if err != nil {
		return fmt.Errorf("embedded PWA: %w", err)
	}

	// Each write to a ledger reaches the open devices and the push
	// notifications through the feed (ADR 0013).
	changes := changefeed.New(cfg.databaseURL, logger)
	go changes.Run(ctx)
	notified := make(chan struct{})
	var pushKey string
	if cfg.vapid != nil {
		pushKey = cfg.vapid.Public
		notifier := push.New(push.Config{DB: db, Sender: push.NewWebPush(*cfg.vapid, cfg.origins[0]), Logger: logger})
		go func() {
			notifier.Run(ctx, changes)
			close(notified)
		}()
	} else {
		close(notified)
	}

	api := httpapi.NewServer(db, version).WithDeps(httpapi.Deps{
		DB:           db,
		Changes:      changes,
		Auth:         authService,
		Origins:      cfg.origins,
		InviteSecret: cfg.inviteSecret,
		HTTPSOrigin:  cfg.httpsOrigin,
		PushKey:      pushKey,
	})
	srv := newHTTPServer(cfg.addr, httpapi.NewHandler(api, pwa), logger)
	// Event streams never end on their own; Shutdown would wait for them.
	srv.RegisterOnShutdown(api.CloseStreams)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	logger.Info("listening", "url", "http://"+cfg.addr, "version", version)

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
	// The notifier sends what was waiting before the database closes.
	<-notified
	return nil
}

// printVAPIDKeys is `quitsie vapid-keys`: it prints a new key pair for push
// notifications, to set in the environment of the server.
func printVAPIDKeys(*slog.Logger) error {
	keys, err := push.GenerateKeys()
	if err != nil {
		return err
	}
	fmt.Printf("QUITSIE_VAPID_PUBLIC_KEY=%s\nQUITSIE_VAPID_PRIVATE_KEY=%s\n", keys.Public, keys.Private)
	return nil
}

// recordRestore is `quitsie record-restore`: it records that the database
// came back from a backup, so the sync versions jump past any the lost copy
// handed out and devices that synced after the backup rebuild their replicas
// (store.RecordRestore). Run it once after a restore, before the server serves
// the database; running it again, or without a restore, is harmless.
func recordRestore(logger *slog.Logger) error {
	url, err := databaseURL(os.Getenv)
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := store.Open(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	gap, err := store.RecordRestore(ctx, db, time.Now())
	if err != nil {
		return err
	}
	logger.Warn("recorded the restore: sync versions jumped, and devices that synced after the backup will rebuild their replicas",
		"restored_version", gap.RestoredVersion, "jumped_version", gap.JumpedVersion)
	return nil
}

// newHTTPServer serves handler on addr.
func newHTTPServer(addr string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// Longer than the slowest pull, with its wait to get in.
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  2 * time.Minute,
		// What net/http logs on its own is a bug in a handler (a panic, a
		// header written twice, a bad Content-Length) or a failure to accept
		// connections, such as running out of file descriptors. All of it
		// needs the owner, and the alerts only read level=ERROR; without
		// this it goes through the log package at INFO. The server speaks
		// plain HTTP to cloudflared, so the TLS handshake errors that
		// scanners cause never reach it.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

// config is the server's configuration, read from the environment.
type config struct {
	addr        string
	databaseURL string
	// origins are the app's own origins; requests that change data must
	// come from one of them.
	origins []string
	// httpsOrigin is the public origin in production, where the server
	// enforces HTTPS; empty in development.
	httpsOrigin string
	// logMail writes emails to the log instead of sending them with resend.
	logMail bool
	resend  mail.ResendConfig
	// mailPerHour and mailPerDay are the budget of login-code emails.
	mailPerHour int
	mailPerDay  int
	// sendCooldown is the wait between login codes of an email; zero means
	// auth's default.
	sendCooldown    time.Duration
	google          *auth.GoogleConfig
	defaultCurrency string
	inviteSecret    []byte
	// inviteSecretGenerated says inviteSecret is a random one for this run.
	inviteSecretGenerated bool
	// vapid signs the push notifications; nil when the server sends none.
	vapid *push.Keys
}

const (
	googleCallbackPath = "/api/v1/auth/google/callback"
	// viteOrigin is the PWA under `npm run dev`, which proxies /api here.
	viteOrigin = "http://localhost:5173"
	// runLocally is the hint of the errors a developer meets first.
	runLocally = "to run locally, set QUITSIE_ENV=development and QUITSIE_MAIL=log"
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
		addr:            or("QUITSIE_ADDR", listenAddr(getenv("PORT"))),
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

	var err error
	if cfg.databaseURL, err = databaseURL(getenv); err != nil {
		return config{}, err
	}

	origin := getenv("QUITSIE_PUBLIC_ORIGIN")
	switch {
	case origin != "":
	case development:
		origin = "http://localhost:8080"
	default:
		return config{}, errors.New("QUITSIE_PUBLIC_ORIGIN is required (" + runLocally + ")")
	}
	if err := checkOrigin(origin, development); err != nil {
		return config{}, err
	}
	cfg.origins = []string{origin}
	if development && origin != viteOrigin {
		cfg.origins = append(cfg.origins, viteOrigin)
	}
	if !development {
		cfg.httpsOrigin = origin
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
			return config{}, errors.New("QUITSIE_RESEND_API_KEY and QUITSIE_MAIL_FROM are required to send login codes (" + runLocally + ")")
		}
	case "log":
		if !development {
			return config{}, errors.New("QUITSIE_MAIL=log writes login codes to the log, so it needs QUITSIE_ENV=development")
		}
		cfg.logMail = true
	default:
		return config{}, fmt.Errorf("QUITSIE_MAIL must be resend or log, not %q", sender)
	}

	if cfg.mailPerHour, err = positive(getenv, "QUITSIE_MAIL_MAX_PER_HOUR", auth.DefaultMailPerHour); err != nil {
		return config{}, err
	}
	if cfg.mailPerDay, err = positive(getenv, "QUITSIE_MAIL_MAX_PER_DAY", auth.DefaultMailPerDay); err != nil {
		return config{}, err
	}
	if cfg.mailPerHour > cfg.mailPerDay {
		return config{}, fmt.Errorf("QUITSIE_MAIL_MAX_PER_HOUR (%d) cannot exceed QUITSIE_MAIL_MAX_PER_DAY (%d)", cfg.mailPerHour, cfg.mailPerDay)
	}
	// Production keeps the minute: a shorter wait lets anyone flood an inbox.
	if v := getenv("QUITSIE_LOGIN_CODE_COOLDOWN"); v != "" {
		if !development {
			return config{}, errors.New("QUITSIE_LOGIN_CODE_COOLDOWN is only for development")
		}
		if cfg.sendCooldown, err = time.ParseDuration(v); err != nil || cfg.sendCooldown <= 0 {
			return config{}, fmt.Errorf("QUITSIE_LOGIN_CODE_COOLDOWN must be a positive duration such as 1ms, not %q", v)
		}
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
	case len(cfg.inviteSecret) >= invites.MinSecretLength:
	case len(cfg.inviteSecret) == 0 && development:
		cfg.inviteSecret = make([]byte, invites.MinSecretLength)
		rand.Read(cfg.inviteSecret)
		cfg.inviteSecretGenerated = true
	default:
		return config{}, fmt.Errorf("QUITSIE_INVITE_SECRET must have at least %d bytes", invites.MinSecretLength)
	}

	keys := push.Keys{Public: getenv("QUITSIE_VAPID_PUBLIC_KEY"), Private: getenv("QUITSIE_VAPID_PRIVATE_KEY")}
	switch {
	case keys.Public == "" && keys.Private == "":
	case keys.Public == "" || keys.Private == "":
		return config{}, errors.New("QUITSIE_VAPID_PUBLIC_KEY and QUITSIE_VAPID_PRIVATE_KEY go together: set both, or neither for no push notifications")
	default:
		if err := keys.Check(); err != nil {
			return config{}, fmt.Errorf("QUITSIE_VAPID_PUBLIC_KEY and QUITSIE_VAPID_PRIVATE_KEY: %w (`quitsie vapid-keys` prints a new pair)", err)
		}
		cfg.vapid = &keys
	}
	return cfg, nil
}

// listenAddr is where the server listens when QUITSIE_ADDR is not set. A
// platform that routes traffic to the container says the port in PORT, and
// reaches it from outside, so the server listens on every interface.
func listenAddr(port string) string {
	if port == "" {
		return "127.0.0.1:8080"
	}
	return ":" + port
}

// developmentDatabase is the PostgreSQL of compose.yaml.
const developmentDatabase = "postgres://quitsie:quitsie@127.0.0.1:5433/quitsie?sslmode=disable"

// databaseURL is QUITSIE_DATABASE_URL, which production requires.
func databaseURL(getenv func(string) string) (string, error) {
	if url := getenv("QUITSIE_DATABASE_URL"); url != "" {
		return url, nil
	}
	if getenv("QUITSIE_ENV") == "development" {
		return developmentDatabase, nil
	}
	return "", errors.New("QUITSIE_DATABASE_URL is required (" + runLocally + ", and start the database with docker compose up -d)")
}

// positive reads the variable key as a whole number of at least 1, or
// fallback if it is not set.
func positive(getenv func(string) string, key string, fallback int) (int, error) {
	v := getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a whole number of at least 1, not %q", key, v)
	}
	return n, nil
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
