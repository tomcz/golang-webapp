package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
	"golang.org/x/sync/errgroup"
)

var (
	env  = flag.String("env", "dev", "Environment name")
	addr = flag.String("addr", ":3000", "Listen address")
)

func main() {
	flag.Parse()
	if err := realMain(); err != nil {
		log.WithError(err).Fatal("application failed")
	}
	log.Info("application stopped")
}

func realMain() error {
	r := http.NewServeMux()
	r.HandleFunc("/index", index)
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound))

	sm := secure.New(secure.Options{
		BrowserXssFilter:     true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		ReferrerPolicy:       "no-referrer",
		SSLRedirect:          true,
		SSLTemporaryRedirect: true,
		IsDevelopment:        *env == "dev",
	})
	h := sm.Handler(r)

	ll := log.WithField("component", "handler")
	h = panicRecovery(r, ll)
	h = requestLogger(h, ll)

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
	return group.Wait()
}

func panicRecovery(h http.Handler, ll log.FieldLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				w.WriteHeader(http.StatusInternalServerError)
				stack := string(debug.Stack())
				ll.WithField("panic", p).WithField("panic_stack", stack).Error("recovered from panic")
			}
		}()
		h.ServeHTTP(w, r)
	})
}

func requestLogger(h http.Handler, ll log.FieldLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := negroni.NewResponseWriter(w)
		h.ServeHTTP(rw, r)

		fields := log.Fields{
			"duration_ms": time.Since(start).Milliseconds(),
			"status":      rw.Status(),
			"size":        rw.Size(),
			"hostname":    r.Host,
			"method":      r.Method,
			"path":        r.URL.Path,
			"referer":     r.Referer(),
			"user_agent":  r.UserAgent(),
		}
		ll.WithFields(fields).Info("request finished")
	})
}

func index(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintln(w, "hello")
}
