package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	"github.com/gorilla/sessions"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
)

type appCfg struct {
	KnownUsers     string        `name:"known-users" env:"KNOWN_USERS" help:"Valid 'user:password,user2:password2,...' combinations."`
	LogLevel       string        `name:"log-level" env:"LOG_LEVEL" default:"info" help:"Logging level (debug, info, warn, error)."`
	LogType        string        `name:"log-type" env:"LOG_TYPE" default:"default" help:"Logger type (default, text, json)."`
	ListenAddr     string        `name:"listen-addr" env:"LISTEN_ADDR" default:":3000" help:"Service 'ip:port' listen address."`
	TlsCertFile    string        `name:"tls-cert" env:"TLS_CERT_FILE" type:"existingfile" help:"For HTTPS service, optional."`
	TlsKeyFile     string        `name:"tls-key" env:"TLS_KEY_FILE" type:"existingfile" help:"For HTTPS service, optional."`
	SessionName    string        `name:"session" env:"SESSION_NAME" default:"_app_session" help:"Name of session cookie."`
	SessionMaxAge  time.Duration `name:"max-age" env:"SESSION_MAX_AGE" default:"24h" help:"MaxAge of session cookie."`
	SessionAuthKey string        `name:"auth-key" env:"SESSION_AUTH_KEY" help:"Session authentication key."`
	SessionEncKey  string        `name:"enc-key" env:"SESSION_ENC_KEY" help:"Session encryption key."`
	Version        bool          `name:"version" short:"v" help:"Show build version and exit."`
	Keygen         bool          `name:"keygen" short:"k" help:"Generate session keys and exit."`
}

func main() {
	var app appCfg
	kong.Parse(&app, kong.Description("Example golang webapp."))
	log := app.setupLogging()

	if app.Version {
		fmt.Println(build.Version())
		os.Exit(0)
	}

	if app.Keygen {
		fmt.Printf("export SESSION_AUTH_KEY=%q\n", randomPassword(log))
		fmt.Printf("export SESSION_ENC_KEY=%q\n", randomPassword(log))
		os.Exit(0)
	}

	if err := app.run(log); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func (a appCfg) run(log *slog.Logger) error {
	useTLS := a.TlsCertFile != "" && a.TlsKeyFile != ""

	handler := handlers.NewHandler(a.parseKnownUsers())
	handler = webapp.WithMiddleware(a.newSessionStore(useTLS), a.SessionName, handler)

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
	logDefaults := []any{"build", build.Version()}
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

func (a appCfg) newSessionStore(useTLS bool) sessions.Store {
	store := sessions.NewCookieStore(sessionKey(a.SessionAuthKey), sessionKey(a.SessionEncKey))
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
	return store
}

func sessionKey(key string) []byte {
	if key != "" {
		buf := sha256.Sum256([]byte(key))
		return buf[:]
	}
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return buf
}

func randomPassword(log *slog.Logger) string {
	pwd, err := password.Generate(64, 10, 0, false, true)
	if err != nil {
		log.Error("keygen failed", "error", err)
		os.Exit(1)
	}
	return pwd
}
