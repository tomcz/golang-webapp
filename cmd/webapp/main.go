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
	"github.com/tomcz/gotools/env"
	"github.com/tomcz/gotools/errgroup"
	"github.com/tomcz/gotools/quiet"

	"github.com/tomcz/golang-webapp/build"
)

type appConfig struct {
	Addr        string `mapstructure:"LISTEN_ADDR"`
	CookieAuth  string `mapstructure:"COOKIE_AUTH_KEY"`
	CookieEnc   string `mapstructure:"COOKIE_ENC_KEY"`
	CookieName  string `mapstructure:"COOKIE_NAME"`
	TlsCertFile string `mapstructure:"TLS_CERT_FILE"`
	TlsKeyFile  string `mapstructure:"TLS_KEY_FILE"`
	Environment string `mapstructure:"ENV"`
}

func main() {
	log := logrus.WithField("build", build.Version())

	cfg := appConfig{
		Addr:        ":3000",
		CookieName:  "example",
		Environment: "development",
	}
	if err := env.PopulateFromEnv(&cfg); err != nil {
		log.WithError(err).Fatalln("configuration failed")
	}

	log = log.WithField("env", cfg.Environment)
	session := newSessionStore(cfg.CookieName, cfg.CookieAuth, cfg.CookieEnc)
	handler := withMiddleware(newHandler(session), log, cfg.Environment == "development")
	server := &http.Server{Addr: cfg.Addr, Handler: handler}

	group, ctx := errgroup.WithContext(context.Background())
	group.Go(func() error {
		var err error
		ll := log.WithField("addr", cfg.Addr)
		if cfg.TlsCertFile != "" && cfg.TlsKeyFile != "" {
			ll.Info("starting server with TLS")
			err = server.ListenAndServeTLS(cfg.TlsCertFile, cfg.TlsKeyFile)
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
			quiet.CloseWithTimeout(server.Shutdown, 100*time.Millisecond)
			return nil
		case <-ctx.Done():
			return nil
		}
	})
	if err := group.Wait(); err != nil {
		log.WithError(err).Fatalln("application failed")
	}
	log.Info("application stopped")
}
