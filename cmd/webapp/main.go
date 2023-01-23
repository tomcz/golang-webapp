package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tomcz/gotools/errgroup"
	"github.com/tomcz/gotools/quiet"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/internal"
)

const development = "development"

var (
	env string
	log logrus.FieldLogger
)

func init() {
	env = getenv("ENV", development)
	if env == development {
		logrus.SetFormatter(&logrus.TextFormatter{})
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	log = logrus.WithFields(logrus.Fields{
		"component": "main",
		"build":     build.Version(),
		"env":       env,
	})
}

func main() {
	if err := realMain(); err != nil {
		log.WithError(err).Fatalln("application failed")
	}
	log.Info("application stopped")
}

func realMain() error {
	addr := getenv("LISTEN_ADDR", ":3000")
	cookieAuth := getenv("COOKIE_AUTH_KEY", "")
	cookieEnc := getenv("COOKIE_ENC_KEY", "")
	cookieName := getenv("COOKIE_NAME", "example")
	tlsCertFile := getenv("TLS_CERT_FILE", "")
	tlsKeyFile := getenv("TLS_KEY_FILE", "")
	withTLS := tlsCertFile != "" && tlsKeyFile != ""

	session := internal.NewSessionStore(cookieName, cookieAuth, cookieEnc)
	handler := internal.WithMiddleware(internal.NewHandler(session), log, withTLS)
	server := &http.Server{Addr: addr, Handler: handler}

	group, ctx := errgroup.NewContext(context.Background())
	group.Go(func() error {
		ll := log.WithField("addr", addr)
		if withTLS {
			ll.Info("starting server with TLS")
			return server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
		}
		ll.Info("starting server without TLS")
		return server.ListenAndServe()
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Info("shutdown received")
			quiet.CloseWithTimeout(server.Shutdown, 100*time.Millisecond)
			return nil
		case <-ctx.Done():
			return nil
		}
	})
	err := group.Wait()
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func getenv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}
