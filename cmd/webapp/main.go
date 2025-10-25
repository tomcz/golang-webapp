package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/gorilla/sessions"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
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
		fmt.Printf("export SESSION_AUTH_KEY=%q\n", base64.StdEncoding.EncodeToString(newSessionKey()))
		fmt.Printf("export SESSION_ENC_KEY=%q\n", base64.StdEncoding.EncodeToString(newSessionKey()))
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

	sessionStore, err := a.newSessionStore(useTLS)
	if err != nil {
		return fmt.Errorf("newSessionStore: %w", err)
	}

	router := webapp.NewRouter(sessionStore, a.SessionName, commit, a.BehindProxy)
	handlers.RegisterRoutes(router, a.parseKnownUsers())

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
		ll := log.With("addr", a.ListenAddr)
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

func (a appCfg) newSessionStore(useTLS bool) (sessions.Store, error) {
	authKey, err := sessionKey(a.SessionAuthKey)
	if err != nil {
		return nil, fmt.Errorf("SessionAuthKey: %w", err)
	}
	encKey, err := sessionKey(a.SessionEncKey)
	if err != nil {
		return nil, fmt.Errorf("SessionEncKey: %w", err)
	}
	store := sessions.NewCookieStore(authKey, encKey)
	maxAge := int(a.SessionMaxAge.Seconds())
	store.Options.MaxAge = maxAge
	store.Options.HttpOnly = true
	store.Options.Path = "/"
	if useTLS {
		store.Options.Secure = true
		store.Options.SameSite = http.SameSiteNoneMode
	} else {
		store.Options.Secure = false
		store.Options.SameSite = http.SameSiteDefaultMode
	}
	store.MaxAge(maxAge)
	return store, nil
}

func sessionKey(key string) ([]byte, error) {
	if key == "" {
		return newSessionKey(), nil
	}
	buf, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	if len(buf) != 32 {
		return nil, errors.New("invalid key length")
	}
	return buf, nil
}

func newSessionKey() []byte {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return buf
}
