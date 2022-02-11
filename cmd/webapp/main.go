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
)

func main() {
	env := flag.String("env", "dev", "Environment name")
	addr := flag.String("addr", ":3000", "Listen address")
	flag.Parse()

	h := withMiddleware(newHandler(), *env)
	s := &http.Server{Addr: *addr, Handler: h}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var group errgroup.Group
	group.Go(func() error {
		defer cancel()
		log.WithField("addr", *addr).Info("starting server")
		err := s.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		log.Info("server stopped")
		return nil
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Info("shutdown received")
			return s.Shutdown(context.Background())
		case <-ctx.Done():
			return nil
		}
	})
	if err := group.Wait(); err != nil {
		log.WithError(err).Fatalln("server failed")
	}
}
