package main

import (
	"net/http"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
)

func withMiddleware(h http.Handler, isDev bool) http.Handler {
	sm := secure.New(secure.Options{
		BrowserXssFilter:     true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		ReferrerPolicy:       "no-referrer",
		SSLRedirect:          true,
		SSLTemporaryRedirect: true,
		IsDevelopment:        isDev,
	})
	h = sm.Handler(h)
	h = panicRecovery(h)
	h = requestLogger(h)
	return traceRequest(h)
}

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				stack := string(debug.Stack())
				fields := log.Fields{"panic": p, "panic_stack": stack}
				rlog(r).WithFields(fields).Error("recovered from panic")
				render500(w, r, "request failed")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := negroni.NewResponseWriter(w)
		next.ServeHTTP(rw, r)

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
		rlog(r).WithFields(fields).Info("request finished")
	})
}

func noStoreCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func staticCacheControl(next http.Handler, isDev bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDev {
			// don't cache locally, so we can work on the content
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			// embedded content can be cached by the browser for 10 minutes
			w.Header().Set("Cache-Control", "private, max-age=600")
		}
		next.ServeHTTP(w, r)
	})
}
