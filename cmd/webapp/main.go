package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/build"
)

func main() {
	env := flag.String("env", "dev", "Environment name")
	addr := flag.String("addr", ":3000", "Listen address")
	cookieAuth := flag.String("cookie-auth", "", "Cookie authentication key")
	cookieEnc := flag.String("cookie-enc", "", "Cookie encryption key")
	cookieName := flag.String("cookie-name", "example", "Cookie name")
	tlsCertFile := flag.String("tls-cert", "", "TLS certificate file")
	tlsKeyFile := flag.String("tls-key", "", "TLS private key file")
	flag.Parse()

	logger := log.WithFields(log.Fields{
		"build": build.Version(),
		"env":   *env,
	})

	isDev := *env == "dev"
	session := newSessionStore(*cookieAuth, *cookieEnc, *cookieName)
	handler := withMiddleware(newHandler(session, isDev), logger, isDev)
	server := &http.Server{Addr: *addr, Handler: handler}

	ctx, cancel := context.WithCancel(context.Background())
	var group errgroup.Group
	group.Go(func() error {
		defer cancel()
		var err error
		ll := logger.WithField("addr", *addr)
		if *tlsCertFile != "" && *tlsKeyFile != "" {
			ll.Info("starting server with TLS")
			err = server.ListenAndServeTLS(*tlsCertFile, *tlsKeyFile)
		} else {
			ll.Info("starting server without TLS")
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		ll.Info("server stopped")
		return nil
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			logger.Info("shutdown received")
			return server.Shutdown(context.Background())
		case <-ctx.Done():
			return nil
		}
	})
	if err := group.Wait(); err != nil {
		logger.WithError(err).Fatalln("application failed")
	}
	logger.Info("application stopped")
}
