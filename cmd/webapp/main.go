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
	"github.com/lmittmann/tint"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/app"
)

// provided by go build
var commit string

type serviceCmd struct {
	KnownUsers     string        `env:"KNOWN_USERS" help:"Valid 'user:password,user2:password2,...' combinations."`
	LogLevel       string        `env:"LOG_LEVEL" default:"info" help:"Logging level (debug, info, warn, error)."`
	LogType        string        `env:"LOG_TYPE" default:"colour" help:"Logger type (default, colour, text, json)."`
	ListenAddr     string        `env:"LISTEN_ADDR" default:":3000" help:"Service 'ip:port' listen address."`
	TlsCertFile    string        `env:"TLS_CERT_FILE" type:"existingfile" help:"For HTTPS service, optional."`
	TlsKeyFile     string        `env:"TLS_KEY_FILE" type:"existingfile" help:"For HTTPS service, optional."`
	SessionName    string        `env:"SESSION_NAME" default:"_app_session" help:"Name of session cookie."`
	SessionMaxAge  time.Duration `env:"SESSION_MAX_AGE" default:"24h" help:"MaxAge of session cookie."`
	SessionAuthKey string        `env:"SESSION_AUTH_KEY" help:"Session authentication key."`
	SessionEncKey  string        `env:"SESSION_ENC_KEY" help:"Session encryption key."`
	BehindProxy    bool          `env:"BEHIND_PROXY" help:"Use HTTP proxy headers."`
}

type keygenCmd struct{}

type versionCmd struct{}

type appCfg struct {
	Service serviceCmd `cmd:"" default:"1" help:"Start the webapp."`
	Version versionCmd `cmd:"" help:"Show build version and exit."`
	Keygen  keygenCmd  `cmd:"" help:"Generate session keys and exit."`
}

func main() {
	cfg := kong.Parse(&appCfg{}, kong.Description("Example golang webapp."))
	if err := cfg.Run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func (*versionCmd) Run() error {
	fmt.Println(commit)
	return nil
}

func (*keygenCmd) Run() error {
	fmt.Printf("export SESSION_AUTH_KEY=%q\n", webapp.NewSessionKey())
	fmt.Printf("export SESSION_ENC_KEY=%q\n", webapp.NewSessionKey())
	return nil
}

func (a *serviceCmd) Run() error {
	log := a.setupLogging()
	useTLS := a.TlsCertFile != "" && a.TlsKeyFile != ""

	sessions, err := webapp.UseSessionCookies(webapp.SessionCookieConfig{
		CookieName: a.SessionName,
		AuthKey:    a.SessionAuthKey,
		EncKey:     a.SessionEncKey,
		MaxAge:     a.SessionMaxAge,
		Secure:     useTLS || a.BehindProxy,
	})
	if err != nil {
		return err
	}
	handler := app.Handler(app.HandlerConfig{
		Sessions:    sessions,
		KnownUsers:  a.parseKnownUsers(),
		Commit:      commit,
		BehindProxy: a.BehindProxy,
	})
	server := &http.Server{
		Addr:              a.ListenAddr,
		Handler:           handler,
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
	err = group.Wait()
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *serviceCmd) parseKnownUsers() map[string]string {
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

func (a *serviceCmd) setupLogging() *slog.Logger {
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
	case "colour":
		opts := &tint.Options{Level: level, TimeFormat: time.TimeOnly, ReplaceAttr: highlightErrors}
		h := tint.NewHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	default:
		slog.SetLogLoggerLevel(level)
		slog.SetDefault(slog.Default().With(logDefaults...))
	}
	return slog.With("component", "main")
}

func highlightErrors(_ []string, attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindAny {
		if _, ok := attr.Value.Any().(error); ok {
			return tint.Attr(9, attr)
		}
	}
	return attr
}
