package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lmittmann/tint"
	"github.com/tomcz/gotools/reloader"
	"github.com/tomcz/gotools/runner"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/app"
)

// provided by go build
var commit string

type serviceCmd struct {
	KnownUsers     map[string]string `env:"KNOWN_USERS" help:"Where KEY is the username and VALUE is the hashed password."`
	LogLevel       slog.Level        `env:"LOG_LEVEL" type:"level" help:"Logging level (debug, info, warn, error)."`
	LogType        string            `env:"LOG_TYPE" default:"colour" enum:"default,colour,text,json" help:"Logger type (default, colour, text, json)."`
	ListenAddr     string            `env:"LISTEN_ADDR" default:"127.0.0.1:3000" help:"Service 'ip:port' listen address."`
	TlsCertFile    string            `env:"TLS_CERT_FILE" placeholder:"FILE" type:"existingfile" help:"For HTTPS service, optional."`
	TlsKeyFile     string            `env:"TLS_KEY_FILE" placeholder:"FILE" type:"existingfile" help:"For HTTPS service, optional."`
	TlsReload      time.Duration     `env:"TLS_RELOAD" help:"Optional interval between TLS file reloads to allow for key rotation"`
	SessionName    string            `env:"SESSION_NAME" default:"_app_session" help:"Name of session cookie."`
	SessionMaxAge  time.Duration     `env:"SESSION_MAX_AGE" default:"24h" help:"MaxAge of session cookie."`
	SessionAuthKey string            `env:"SESSION_AUTH_KEY" help:"Session authentication key."`
	SessionEncKey  string            `env:"SESSION_ENC_KEY" help:"Session encryption key."`
	BehindProxy    bool              `env:"BEHIND_PROXY" help:"Use HTTP proxy headers."`
}

type versionCmd struct{}

type keygenCmd struct{}

type passwordCmd struct{}

type appCfg struct {
	Service  serviceCmd  `cmd:"" default:"1" help:"Start the webapp."`
	Version  versionCmd  `cmd:"" help:"Show build version and exit."`
	Keygen   keygenCmd   `cmd:"" help:"Generate session keys and exit."`
	Password passwordCmd `cmd:"" help:"Create hash for a password."`
}

func main() {
	opts := []kong.Option{
		kong.Description("Example golang webapp."),
		kong.NamedMapper("level", kong.MapperFunc(levelMapper)),
		kong.HelpOptions{Compact: true},
	}
	cfg := kong.Parse(&appCfg{}, opts...)
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

func (*passwordCmd) Run() error {
	fmt.Print("Password to hash: ")
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return errors.New("empty password")
	}
	hashed := webapp.HashPassword(password)
	fmt.Println("Hashed password:", hashed)
	return nil
}

func (s *serviceCmd) Run() error {
	log := s.setupLogging() // setup first so that failure messages are properly logged

	sessions, err := webapp.UseSessionCookies(webapp.SessionCookieConfig{
		CookieName: s.SessionName,
		AuthKey:    s.SessionAuthKey,
		EncKey:     s.SessionEncKey,
		MaxAge:     s.SessionMaxAge,
		Secure:     s.useTLS() || s.BehindProxy,
	})
	if err != nil {
		return err
	}

	handler := app.Handler(app.HandlerConfig{
		Sessions:    sessions,
		KnownUsers:  s.KnownUsers,
		Commit:      commit,
		BehindProxy: s.BehindProxy,
	})
	server := &http.Server{
		Addr:              s.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: time.Minute,
		// Consider setting ReadTimeout, WriteTimeout, and IdleTimeout
		// to prevent connections from taking resources indefinitely.
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := runner.New()
	service.CleanupTimeout(server.Shutdown, 100*time.Millisecond)
	service.Run(func() error { return s.runServer(ctx, server, log) })
	return service.Wait()
}

func (s *serviceCmd) useTLS() bool {
	return s.TlsCertFile != "" && s.TlsKeyFile != ""
}

func (s *serviceCmd) runServer(ctx context.Context, server *http.Server, log *slog.Logger) error {
	log = log.With("addr", s.ListenAddr, "proxy", s.BehindProxy)
	var err error
	if s.useTLS() {
		err = s.runServerTLS(ctx, server, log)
	} else {
		log.Info("ListenAndServe")
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *serviceCmd) runServerTLS(ctx context.Context, server *http.Server, log *slog.Logger) error {
	server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	if s.TlsReload < time.Minute {
		log.Info("ListenAndServeTLS")
		return server.ListenAndServeTLS(s.TlsCertFile, s.TlsKeyFile)
	}
	loader, err := reloader.New(ctx, s.TlsCertFile, s.TlsKeyFile, s.TlsReload)
	if err != nil {
		return err
	}
	loader.SetLogger(slog.With("component", "tls"))
	log.Info("ListenAndServeTLS", "tls_reload", s.TlsReload.String())
	server.TLSConfig.GetCertificate = loader.GetCertificate
	return server.ListenAndServeTLS("", "")
}

func levelMapper(c *kong.DecodeContext, target reflect.Value) error {
	var value string
	err := c.Scan.PopValueInto("level", &value)
	if err != nil {
		return err
	}
	var level slog.Level
	err = level.UnmarshalText([]byte(value))
	if err != nil {
		return err
	}
	target.Set(reflect.ValueOf(level))
	return nil
}

func (s *serviceCmd) setupLogging() *slog.Logger {
	logDefaults := []any{"build", commit}
	switch s.LogType {
	case "text":
		opts := &slog.HandlerOptions{Level: s.LogLevel}
		h := slog.NewTextHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	case "json":
		opts := &slog.HandlerOptions{Level: s.LogLevel}
		h := slog.NewJSONHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	case "colour":
		opts := &tint.Options{Level: s.LogLevel, TimeFormat: time.TimeOnly, ReplaceAttr: highlightErrors}
		h := tint.NewHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	default:
		slog.SetLogLoggerLevel(s.LogLevel)
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
