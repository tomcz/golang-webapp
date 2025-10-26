package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/app"
)

// provided by go build
var commit string

type appCfg struct {
	KnownUsers     string        `env:"KNOWN_USERS" help:"Valid 'user:password,user2:password2,...' combinations."`
	LogLevel       string        `env:"LOG_LEVEL" default:"info" help:"Logging level (debug, info, warn, error)."`
	LogType        string        `env:"LOG_TYPE" default:"default" help:"Logger type (default, text, json)."`
	ListenAddr     string        `env:"LISTEN_ADDR" default:":3000" help:"Service 'ip:port' listen address."`
	TlsCertFile    string        `env:"TLS_CERT_FILE" type:"existingfile" help:"For HTTPS service, optional."`
	TlsKeyFile     string        `env:"TLS_KEY_FILE" type:"existingfile" help:"For HTTPS service, optional."`
	SessionName    string        `env:"SESSION_NAME" default:"_app_session" help:"Name of session cookie."`
	SessionMaxAge  time.Duration `env:"SESSION_MAX_AGE" default:"24h" help:"MaxAge of session cookie."`
	SessionAuthKey string        `env:"SESSION_AUTH_KEY" help:"Session authentication key."`
	SessionEncKey  string        `env:"SESSION_ENC_KEY" help:"Session encryption key."`
	BehindProxy    bool          `env:"BEHIND_PROXY" help:"Use HTTP proxy headers."`
	Version        bool          `short:"v" help:"Show build version and exit."`
	Keygen         bool          `short:"k" help:"Generate session keys and exit."`
}

func main() {
	var app appCfg
	kong.Parse(&app, kong.Description("Example golang webapp."))

	if app.Version {
		fmt.Println(commit)
		os.Exit(0)
	}

	if app.Keygen {
		fmt.Printf("export SESSION_AUTH_KEY=%q\n", webapp.NewSessionKey())
		fmt.Printf("export SESSION_ENC_KEY=%q\n", webapp.NewSessionKey())
		os.Exit(0)
	}

	log := app.setupLogging()
	if err := app.Run(log); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func (a appCfg) Run(log *slog.Logger) error {
	useTLS := a.TlsCertFile != "" && a.TlsKeyFile != ""

	sessions := webapp.UseSessionCookies(webapp.SessionCookieConfig{
		CookieName: a.SessionName,
		AuthKey:    a.SessionAuthKey,
		EncKey:     a.SessionEncKey,
		MaxAge:     a.SessionMaxAge,
		Secure:     useTLS || a.BehindProxy,
	})

	router := webapp.NewRouter(sessions, a.BehindProxy, commit)
	app.RegisterRoutes(router, a.parseKnownUsers())

	server := &http.Server{
		Addr:              a.ListenAddr,
		Handler:           router.Handler(),
		ReadHeaderTimeout: time.Minute,
		// Consider setting ReadTimeout, WriteTimeout, and IdleTimeout
		// to prevent connections from taking resources indefinitely.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		ll := log.With("addr", a.ListenAddr, "proxy", a.BehindProxy)
		if useTLS {
			ll.Info("starting server with TLS")
			server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
			return server.ListenAndServeTLS(a.TlsCertFile, a.TlsKeyFile)
		}
		ll.Info("starting server without TLS")
		return server.ListenAndServe()
	})
	group.Go(func() error {
		<-ctx.Done()
		log.Info("stopping server")
		timeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		return server.Shutdown(timeout)
	})
	err := group.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a appCfg) parseKnownUsers() map[string]string {
	if a.KnownUsers == "" {
		return nil
	}
	users := map[string]string{}
	for token := range strings.SplitSeq(a.KnownUsers, ",") {
		before, after, found := strings.Cut(token, ":")
		if found {
			users[before] = after
		}
	}
	return users
}

func (a appCfg) setupLogging() *slog.Logger {
	var level slog.Level
	switch a.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logDefaults := []any{"build", commit}
	switch a.LogType {
	case "text":
		opts := &slog.HandlerOptions{Level: level}
		h := slog.NewTextHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	case "json":
		opts := &slog.HandlerOptions{Level: level}
		h := slog.NewJSONHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	default:
		slog.SetLogLoggerLevel(level)
		slog.SetDefault(slog.Default().With(logDefaults...))
	}
	return slog.With("component", "main")
}
