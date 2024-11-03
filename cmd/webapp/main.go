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

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions/cookie"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions/redis"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions/sqlite"
)

var (
	knownUsers = envFlag("known-users", "KNOWN_USERS", "", "Valid 'user:password,user2:password2,...' combinations")

	logLevel = envFlag("log-level", "LOG_LEVEL", "info", "Logging level (debug, info, warn)")
	logType  = envFlag("log-type", "LOG_TYPE", "default", "Logger type (default, text, json)")

	listenAddr  = envFlag("listen-addr", "LISTEN_ADDR", ":3000", "Service 'ip:port' listen address")
	tlsCertFile = envFlag("tls-cert", "TLS_CERT_FILE", "", "For HTTPS service, optional")
	tlsKeyFile  = envFlag("tls-key", "TLS_KEY_FILE", "", "For HTTPS service, optional")

	sessionName  = envFlag("session-name", "SESSION_NAME", "_app_session", "Name of HTTP application cookie")
	sessionStore = envFlag("session-store", "SESSION_STORE", "sqlite", "Store type (sqlite, redis, cookie)")
	cookieCipher = envFlag("session-cipher", "SESSION_CIPHER", "", "Cookie cipher key; if not provided a random one will be used")
	dbFile       = envFlag("session-sqlite", "SESSION_SQLITE", defaultDatabaseFile(), "sqlite session store file")
	redisAddr    = envFlag("session-redis-addr", "SESSION_REDIS_ADDR", "127.0.0.1:6379", "Redis host:port")
	redisUser    = envFlag("session-redis-user", "SESSION_REDIS_USER", "", "Redis username, optional")
	redisPass    = envFlag("session-redis-pass", "SESSION_REDIS_PASS", "", "Redis password, optional")
	redisTLS     = envFlag("session-redis-tls", "SESSION_REDIS_TLS", "off", "Redis TLS (off, on, insecure)")

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

func defaultDatabaseFile() string {
	pname, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.db", pname)
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

	log, err := setupLogging()
	if err != nil {
		slog.Error("logging setup failed", "error", err)
		os.Exit(1)
	}
	if err = realMain(log); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func realMain(log *slog.Logger) error {
	withTLS := *tlsCertFile != "" && *tlsKeyFile != ""

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	codec, err := createCodec(ctx)
	if err != nil {
		return err
	}
	defer codec.Close()

	session := webapp.NewSessionWrapper(*sessionName, codec, webapp.CsrfPerSession)
	handler := webapp.WithMiddleware(handlers.NewHandler(session, parseKnownUsers()), withTLS)

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: handler,
		// Consider setting ReadTimeout, WriteTimeout, and IdleTimeout
		// to prevent connections from taking resources indefinitely.
	}
	if withTLS {
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	}

	go func() {
		<-ctx.Done()
		log.Info("shutdown received")
		_ = server.Shutdown(context.Background())
	}()

	ll := log.With("addr", *listenAddr, "sessions", *sessionStore)
	if withTLS {
		ll.Info("starting server with TLS")
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

func createCodec(ctx context.Context) (webapp.SessionCodec, error) {
	switch *sessionStore {
	case "sqlite":
		return sqlite.New(ctx, *dbFile)
	case "redis":
		return redis.New(*redisAddr, *redisUser, *redisPass, *redisTLS), nil
	case "cookie":
		return cookie.New(*cookieCipher)
	default:
		return nil, fmt.Errorf("unknown session store type: %q", *sessionStore)
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

func setupLogging() (*slog.Logger, error) {
	level := slog.LevelInfo
	//goland:noinspection GoDfaNilDereference
	err := level.UnmarshalText([]byte(*logLevel))
	if err != nil {
		return nil, fmt.Errorf("bad LOG_LEVEL: %w", err)
	}
	logDefaults := []any{"build", build.Version()}
	switch strings.ToUpper(*logType) {
	case "TEXT":
		opts := &slog.HandlerOptions{Level: level}
		h := slog.NewTextHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	case "JSON":
		opts := &slog.HandlerOptions{Level: level}
		h := slog.NewJSONHandler(os.Stderr, opts)
		slog.SetDefault(slog.New(h).With(logDefaults...))
	default:
		slog.SetLogLoggerLevel(level)
		slog.SetDefault(slog.Default().With(logDefaults...))
	}
	return slog.With("component", "main"), nil
}
