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
)

const development = "development"

var env string

func init() {
	env = osLookupEnv("ENV", development)
	if env == development {
		logrus.SetFormatter(&logrus.TextFormatter{})
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
}

func main() {
	log := logrus.WithFields(logrus.Fields{
		"build": build.Version(),
		"env":   env,
	})
	if err := realMain(log); err != nil {
		log.WithError(err).Fatalln("application failed")
	}
	log.Info("application stopped")
}

func realMain(log logrus.FieldLogger) error {
	addr := osLookupEnv("LISTEN_ADDR", ":3000")
	cookieAuth := osLookupEnv("COOKIE_AUTH_KEY", "")
	cookieEnc := osLookupEnv("COOKIE_ENC_KEY", "")
	cookieName := osLookupEnv("COOKIE_NAME", "example")
	tlsCertFile := osLookupEnv("TLS_CERT_FILE", "")
	tlsKeyFile := osLookupEnv("TLS_KEY_FILE", "")

	session := newSessionStore(cookieName, cookieAuth, cookieEnc)
	handler := withMiddleware(newHandler(session), log, env == development)
	server := &http.Server{Addr: addr, Handler: handler}

	group, ctx := errgroup.NewContext(context.Background())
	group.Go(func() error {
		ll := log.WithField("addr", addr)
		if tlsCertFile != "" && tlsKeyFile != "" {
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

func osLookupEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
