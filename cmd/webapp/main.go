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
	"syscall"

	"github.com/ianschenck/envflag"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
)

const development = "development"

var (
	env         = envflag.String("ENV", development, "Runtime environment (development or production)")
	logLevel    = envflag.String("LOG_LEVEL", "INFO", "Logging level (DEBUG, INFO, WARN)")
	knownUsers  = envflag.String("KNOWN_USERS", "", "Valid (user:password,user2:password2,...) combinations")
	listenAddr  = envflag.String("LISTEN_ADDR", ":3000", "Bind address")
	cookieEnc   = envflag.String("COOKIE_ENC_KEY", "", "If not provided a random one will be used")
	cookieName  = envflag.String("COOKIE_NAME", "example", "Name of HTTP application cookie")
	tlsCertFile = envflag.String("TLS_CERT_FILE", "", "For HTTPS service, optional")
	tlsKeyFile  = envflag.String("TLS_KEY_FILE", "", "For HTTPS service, optional")
	keygen      = flag.Bool("keygen", false, "Print out a new COOKIE_ENC_KEY and exit")
	log         = slog.Default()
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "Environment variables:\n")
		envflag.PrintDefaults()
	}
}

func main() {
	envflag.Parse()
	flag.Parse()

	if *keygen {
		key, err := webapp.RandomKey()
		if err != nil {
			log.Error("keygen failed", "error", err)
			os.Exit(1)
		}
		fmt.Println(key)
		os.Exit(0)
	}

	log = setupLogging()
	if err := realMain(); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func realMain() error {
	withTLS := *tlsCertFile != "" && *tlsKeyFile != ""

	session, err := webapp.NewSessionStore(*cookieName, *cookieEnc, webapp.CsrfPerSession)
	if err != nil {
		return err
	}
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
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
		<-signalCh
		log.Info("shutdown received")
		server.Shutdown(context.Background()) //nolint:errcheck
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
	level := slog.LevelInfo
	level.UnmarshalText([]byte(*logLevel)) //nolint:errcheck

	var opts slog.HandlerOptions
	opts.Level = level

	var h slog.Handler
	if *env == development {
		h = slog.NewTextHandler(os.Stderr, &opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, &opts)
	}

	slog.SetDefault(slog.New(h).With("env", *env, "build", build.Version()))
	return slog.With("component", "main")
}
