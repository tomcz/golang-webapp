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
	"github.com/tomcz/golang-webapp/internal/webapp/cookie"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
	"github.com/tomcz/golang-webapp/internal/webapp/redis"
)

var (
	knownUsers = envFlag("known-users", "KNOWN_USERS", "", "Valid 'user:password,user2:password2,...' combinations")

	logLevel = envFlag("log-level", "LOG_LEVEL", "info", "Logging level (debug, info, warn)")
	logType  = envFlag("log-type", "LOG_TYPE", "default", "Logger type (default, text, json)")

	listenAddr  = envFlag("listen-addr", "LISTEN_ADDR", ":3000", "Service 'ip:port' listen address")
	tlsCertFile = envFlag("tls-cert", "TLS_CERT_FILE", "", "For HTTPS service, optional")
	tlsKeyFile  = envFlag("tls-key", "TLS_KEY_FILE", "", "For HTTPS service, optional")

	cookieName = envFlag("cookie-name", "COOKIE_NAME", "_app_session", "Name of HTTP application cookie")
	cookieKey  = envFlag("cookie-key", "COOKIE_KEY", "", "If not provided a random one will be used")

	redisAddr = envFlag("redis-addr", "REDIS_ADDR", "", "Redis host:port, optional")
	redisUser = envFlag("redis-user", "REDIS_USER", "", "Redis username, optional")
	redisPass = envFlag("redis-pass", "REDIS_PASS", "", "Redis password, optional")
	redisTLS  = envFlag("redis-tls", "REDIS_TLS", "off", "Redis TLS (off, on, insecure)")

	keygen  = flag.Bool("keygen", false, "Print out a new COOKIE_KEY and exit")
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
		fmt.Println(cookie.RandomKey())
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

	codec, err := createCodec(log)
	if err != nil {
		return err
	}
	defer codec.Close()

	session := webapp.NewSessionWrapper(*cookieName, codec, webapp.CsrfPerSession)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutdown received")
		_ = server.Shutdown(context.Background())
	}()

	ll := log.With("addr", *listenAddr)
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

func createCodec(log *slog.Logger) (webapp.SessionCodec, error) {
	if *redisAddr != "" {
		log.Info("storing sessions in redis")
		return redis.New(*redisAddr, *redisUser, *redisPass, *redisTLS), nil
	}
	log.Info("storing sessions in cookies")
	return cookie.New(*cookieKey)
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
