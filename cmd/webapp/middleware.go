package main

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
)

const (
	reqIdKey = "req_id"
	reqMdKey = "req_md"
)

func withMiddleware(h http.Handler, logger log.FieldLogger, isDev bool) http.Handler {
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
	h = breaker.Handler(breaker.NewBreaker(0.1), breaker.DefaultStatusCodeValidator, h)
	return requestLogger(h, logger)
}

func requestLogger(next http.Handler, logger log.FieldLogger) http.Handler {
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
		fields["req_start_at"] = start
		fields["res_duration_ms"] = duration.Milliseconds()
		fields["res_duration_ns"] = duration.Nanoseconds()
		fields["res_status"] = rw.Status()
		fields["res_size"] = rw.Size()
		fields["req_host"] = r.Host
		fields["req_method"] = r.Method
		fields["req_path"] = r.URL.Path
		fields["req_user_agent"] = r.UserAgent()
		fields["req_remote_addr"] = r.RemoteAddr

		if referer := r.Referer(); referer != "" {
			fields["req_referer"] = referer
		}
		if loc := rw.Header().Get("Location"); loc != "" {
			fields["res_location"] = loc
		}
		logger.WithFields(fields).Info("request finished")
	})
}

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				stack := string(debug.Stack())
				rset(r, "panic_stack", stack)
				rerr(r, fmt.Errorf("panic: %v", p))
				render500(w, r, "Request failed")
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

func setCurrentRouteName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			if name := route.GetName(); name != "" {
				rset(r, "req_route", name)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func rerr(r *http.Request, err error) {
	rset(r, "err", err)
}

func rset(r *http.Request, key string, value interface{}) {
	if md, ok := r.Context().Value(reqMdKey).(log.Fields); ok {
		md[key] = value
	}
}

func render500(w http.ResponseWriter, r *http.Request, msg string) {
	if id, ok := r.Context().Value(reqIdKey).(string); ok {
		msg = fmt.Sprintf("ID: %s\nError: %s\n", id, msg)
	}
	http.Error(w, msg, http.StatusInternalServerError)
}
