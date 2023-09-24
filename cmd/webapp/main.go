package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
)

const development = "development"

var (
	env string
	log *slog.Logger
)

func init() {
	env = getenv("ENV", development)
	isDebug := getenv("LOG_DEBUG", "no") == "yes"
	var opts slog.HandlerOptions
	if isDebug {
		opts.Level = slog.LevelDebug
	} else {
		opts.Level = slog.LevelInfo
	}
	var h slog.Handler
	if env == development {
		h = slog.NewTextHandler(os.Stderr, &opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, &opts)
	}
	slog.SetDefault(slog.New(h).With("env", env, "build", build.Version()))
	log = slog.With("component", "main")
}

func main() {
	if err := realMain(); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func realMain() error {
	knownUsers := getenv("KNOWN_USERS", "")
	addr := getenv("LISTEN_ADDR", ":3000")
	cookieEnc := getenv("COOKIE_ENC_KEY", "")
	cookieName := getenv("COOKIE_NAME", "example")
	tlsCertFile := getenv("TLS_CERT_FILE", "")
	tlsKeyFile := getenv("TLS_KEY_FILE", "")
	withTLS := tlsCertFile != "" && tlsKeyFile != ""

	session, err := webapp.NewSessionStore(cookieName, cookieEnc, webapp.CsrfPerRequest)
	if err != nil {
		return err
	}
	handler := webapp.WithMiddleware(handlers.NewHandler(session, parseKnownUsers(knownUsers)), withTLS)

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		// Consider setting ReadTimeout, WriteTimeout, and IdleTimeout
		// to prevent connections from taking resources indefinitely.
	}
	if withTLS {
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	}
	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
		<-signalCh
		log.Info("shutdown received")
		server.Shutdown(context.Background())
	}()
	ll := log.With("addr", addr)
	if withTLS {
		ll.Info("starting server with TLS")
		err = server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	} else {
		ll.Info("starting server without TLS")
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func parseKnownUsers(value string) map[string]string {
	if value == "" {
		return nil
	}
	knownUsers := map[string]string{}
	for _, token := range strings.Split(value, ",") {
		tuple := strings.SplitN(token, ":", 2)
		if len(tuple) != 2 {
			continue
		}
		knownUsers[tuple[0]] = tuple[1]
	}
	return knownUsers
}

func getenv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}
