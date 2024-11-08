package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions/cookie"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions/memcache"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions/memory"
)

var (
	knownUsers = envFlag("known-users", "KNOWN_USERS", "", "Valid 'user:password,user2:password2,...' combinations")

	logLevel = envFlag("log-level", "LOG_LEVEL", "info", "Logging level (debug, info, warn, error)")
	logType  = envFlag("log-type", "LOG_TYPE", "default", "Logger type (default, text, json)")

	listenAddr  = envFlag("listen-addr", "LISTEN_ADDR", ":3000", "Service 'ip:port' listen address")
	tlsCertFile = envFlag("tls-cert", "TLS_CERT_FILE", "", "For HTTPS service, optional")
	tlsKeyFile  = envFlag("tls-key", "TLS_KEY_FILE", "", "For HTTPS service, optional")

	sessionName  = envFlag("session-name", "SESSION_NAME", "_app_session", "Name of HTTP application cookie")
	sessionStore = envFlag("session-store", "SESSION_STORE", "memory", "Store type (memory, memcached, cookie)")
	cookieCipher = envFlag("session-cipher", "SESSION_CIPHER", "", "Cookie cipher key; if not provided a random one will be used")
	memcacheAddr = envFlag("session-memcached", "SESSION_MEMCACHED", "127.0.0.1:11211", "Memcached server host:port")

	keygen  = flag.Bool("keygen", false, "Print out a new SESSION_CIPHER and exit")
	version = flag.Bool("version", false, "Show build version and exit")
)

func envFlag(flagName, envName, defaultValue, usage string) *string {
	value := os.Getenv(envName)
	if value == "" {
		value = defaultValue
	}
	flagUsage := fmt.Sprintf("%s [$%s]", usage, envName)
	return flag.String(flagName, value, flagUsage)
}

func main() {
	flag.Parse()

	if *keygen {
		fmt.Println(sessions.RandomKey())
		os.Exit(0)
	}

	if *version {
		fmt.Println(build.Version())
		os.Exit(0)
	}

	log := setupLogging()
	if err := realMain(log); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func realMain(log *slog.Logger) error {
	withTLS := *tlsCertFile != "" && *tlsKeyFile != ""

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	store, err := createSessionStore()
	if err != nil {
		return err
	}
	defer store.Close()

	session := webapp.NewSessionWrapper(*sessionName, store, webapp.CsrfPerSession)
	handler := webapp.WithMiddleware(handlers.NewHandler(session, parseKnownUsers()), withTLS)

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: time.Minute,
	}

	go func() {
		<-ctx.Done()
		log.Info("shutdown received")
		_ = server.Shutdown(context.Background())
	}()

	ll := log.With("addr", *listenAddr, "sessions", *sessionStore)
	if withTLS {
		ll.Info("starting server with TLS")
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
		err = server.ListenAndServeTLS(*tlsCertFile, *tlsKeyFile)
	} else {
		ll.Info("starting server without TLS")
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func createSessionStore() (webapp.SessionStore, error) {
	switch *sessionStore {
	case "memory":
		return memory.New(), nil
	case "memcached":
		return memcache.New(*memcacheAddr), nil
	case "cookie":
		return cookie.New(*cookieCipher)
	default:
		return nil, fmt.Errorf("unknown session store: %q", *sessionStore)
	}
}

func parseKnownUsers() map[string]string {
	if *knownUsers == "" {
		return nil
	}
	users := map[string]string{}
	for _, token := range strings.Split(*knownUsers, ",") {
		tuple := strings.SplitN(token, ":", 2)
		if len(tuple) != 2 {
			continue
		}
		users[tuple[0]] = tuple[1]
	}
	return users
}

func setupLogging() *slog.Logger {
	var level slog.Level
	switch *logLevel {
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
	switch *logType {
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
