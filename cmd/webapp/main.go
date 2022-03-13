package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/build"
)

func main() {
	env := osLookupEnv("ENV", "dev")
	addr := osLookupEnv("LISTEN_ADDR", ":3000")
	cookieAuth := osLookupEnv("COOKIE_AUTH_KEY", "")
	cookieEnc := osLookupEnv("COOKIE_ENC_KEY", "")
	cookieName := osLookupEnv("COOKIE_NAME", "example")
	tlsCertFile := osLookupEnv("TLS_CERT_FILE", "")
	tlsKeyFile := osLookupEnv("TLS_KEY_FILE", "")

	log := logrus.WithFields(logrus.Fields{
		"build": build.Version(),
		"env":   env,
	})

	session := newSessionStore(cookieAuth, cookieEnc, cookieName)
	handler := withMiddleware(newHandler(session), log, env == "dev")
	server := &http.Server{Addr: addr, Handler: handler}

	group, ctx := errgroup.WithContext(context.Background())
	group.Go(func() error {
		var err error
		ll := log.WithField("addr", addr)
		if tlsCertFile != "" && tlsKeyFile != "" {
			ll.Info("starting server with TLS")
			err = server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
		} else {
			ll.Info("starting server without TLS")
			err = server.ListenAndServe()
		}
		if errors.Is(err, http.ErrServerClosed) {
			ll.Info("server stopped")
			return nil
		}
		return err
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Info("shutdown received")
			return server.Shutdown(context.Background())
		case <-ctx.Done():
			return nil
		}
	})
	if err := group.Wait(); err != nil {
		log.WithError(err).Fatalln("application failed")
	}
	log.Info("application stopped")
}

func osLookupEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
