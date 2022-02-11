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
	flag.Parse()

	isDev := *env == "dev"
	session := newSessionStore(*cookieAuth, *cookieEnc, *cookieName)
	handler := withMiddleware(newHandler(session, isDev), isDev)
	server := &http.Server{Addr: *addr, Handler: handler}

	ll := log.WithFields(log.Fields{
		"build": build.Version(),
		"addr":  *addr,
		"env":   *env,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var group errgroup.Group
	group.Go(func() error {
		defer cancel()
		ll.Info("starting server")
		err := server.ListenAndServe()
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
			ll.Info("shutdown received")
			return server.Shutdown(context.Background())
		case <-ctx.Done():
			return nil
		}
	})
	if err := group.Wait(); err != nil {
		ll.WithError(err).Fatalln("server failed")
	}
}
