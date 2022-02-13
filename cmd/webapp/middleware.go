package main

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
)

const (
	reqIdKey = "req_id"
	reqMdKey = "req_md"
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
	return requestLogger(h)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fields := log.Fields{}
		reqID := uuid.New().String()

		rw := negroni.NewResponseWriter(w)
		ctx := context.WithValue(r.Context(), reqIdKey, reqID)
		ctx = context.WithValue(ctx, reqMdKey, fields)

		next.ServeHTTP(rw, r.WithContext(ctx))
		duration := time.Since(start)

		fields["req_id"] = reqID
		fields["start_at"] = start
		fields["duration_ms"] = duration.Milliseconds()
		fields["status"] = rw.Status()
		fields["size"] = rw.Size()
		fields["hostname"] = r.Host
		fields["method"] = r.Method
		fields["path"] = r.URL.Path
		fields["user_agent"] = r.UserAgent()

		if referer := r.Referer(); referer != "" {
			fields["referer"] = referer
		}
		if location := rw.Header().Get("Location"); location != "" {
			fields["location"] = location
		}
		log.WithFields(fields).Info("request finished")
	})
}

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				render500(w, r, "request failed")
				stack := string(debug.Stack())
				radd(r, "panic_stack", stack)
				radd(r, "panic", p)
			}
		}()
		next.ServeHTTP(w, r)
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

func radd(r *http.Request, key string, value interface{}) {
	if md, ok := r.Context().Value(reqMdKey).(log.Fields); ok {
		md[key] = value
	}
}

func rmsg(r *http.Request, msg string) string {
	if id, ok := r.Context().Value(reqIdKey).(string); ok {
		return id + ": " + msg
	}
	return msg
}

func render500(w http.ResponseWriter, r *http.Request, msg string) {
	http.Error(w, rmsg(r, msg), http.StatusInternalServerError)
}
