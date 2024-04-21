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

	"github.com/ianschenck/envflag"
	"github.com/tomcz/gotools/quiet"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/handlers"
)

const envDevelopment = "development"

var (
	env         = envflag.String("ENV", envDevelopment, "Runtime environment (development or production)")
	logLevel    = envflag.String("LOG_LEVEL", "INFO", "Logging level (DEBUG, INFO, WARN)")
	logType     = envflag.String("LOG_TYPE", "DEFAULT", "Logger type (DEFAULT, TEXT, JSON)")
	knownUsers  = envflag.String("KNOWN_USERS", "", "Valid 'user:password,user2:password2,...' combinations")
	listenAddr  = envflag.String("LISTEN_ADDR", ":3000", "Service 'ip:port' listen address")
	cookieName  = envflag.String("COOKIE_NAME", "_app_session", "Name of HTTP application cookie")
	cookieAuth  = envflag.String("COOKIE_AUTH_KEY", "", "If not provided a random one will be used")
	cookieEnc   = envflag.String("COOKIE_ENC_KEY", "", "If not provided a random one will be used")
	tlsCertFile = envflag.String("TLS_CERT_FILE", "", "For HTTPS service, optional")
	tlsKeyFile  = envflag.String("TLS_KEY_FILE", "", "For HTTPS service, optional")
	keygen      = flag.Bool("keygen", false, "Print out a new COOKIE_ENC_KEY and exit")
	version     = flag.Bool("version", false, "Show build version and exit")
)

var log *slog.Logger

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

	var err error
	log, err = setupLogging()
	if err != nil {
		slog.Error("logging setup failed", "error", err)
		os.Exit(1)
	}

	if *keygen {
		fmt.Println("COOKIE_AUTH_KEY:", webapp.EncodeRandomKey())
		fmt.Println("COOKIE_ENC_KEY: ", webapp.EncodeRandomKey())
		os.Exit(0)
	}

	if *version {
		fmt.Println(build.Version())
		os.Exit(0)
	}

	if err = realMain(); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func realMain() error {
	withTLS := *tlsCertFile != "" && *tlsKeyFile != ""

	session, err := webapp.NewSessionStore(*cookieName, *cookieAuth, *cookieEnc, webapp.CsrfPerSession)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutdown received")
		quiet.CloseWithTimeout(server.Shutdown, 100*time.Millisecond)
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

func setupLogging() (*slog.Logger, error) {
	level := slog.LevelInfo
	//goland:noinspection GoDfaNilDereference
	err := level.UnmarshalText([]byte(*logLevel))
	if err != nil {
		return nil, fmt.Errorf("bad LOG_LEVEL: %w", err)
	}
	logDefaults := []any{"env", *env, "build", build.Version()}
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
