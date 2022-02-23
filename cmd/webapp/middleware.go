package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
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
	h = breaker.Handler(breaker.NewBreaker(0.1), breaker.DefaultStatusCodeValidator, h)
	// We want to trace every request, whether matched by handler.go or not,
	// and we want to capture all panics and circuit breaker actions, so we're
	// using the otelhttp handler and not otel's gorilla/mux middleware.
	return otelhttp.NewHandler(h, "handler")
}

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				var err error
				if e, ok := p.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("%v", p)
				}
				errID := newErrorID()
				span := trace.SpanFromContext(r.Context())
				span.RecordError(err,
					trace.WithStackTrace(true),
					trace.WithAttributes(attribute.String("err_id", errID)),
					trace.WithAttributes(attribute.String("err_msg", "recovered from panic")),
				)
				msg := fmt.Sprintf("[%s] request failed", errID)
				http.Error(w, msg, http.StatusInternalServerError)
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

func setCurrentRouteAttributes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			span := trace.SpanFromContext(r.Context())
			if name := route.GetName(); name != "" {
				// Technically-speaking this should be the URL http server name,
				// but otelhttp uses the name of the operation (i.e. "handler"),
				// so let's set it to the name of the matched gorilla/mux route.
				span.SetAttributes(semconv.HTTPServerNameKey.String(name))
			}
			if tmpl, err := route.GetPathTemplate(); err == nil {
				// These aren't in the spec format of /path/:id, but since we're
				// matching with gorilla/mux we can only provide what we have.
				span.SetAttributes(semconv.HTTPRouteKey.String(tmpl))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func renderError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	errID := recordError(r, err, msg)
	message := fmt.Sprintf("[%s] %s", errID, msg)
	http.Error(w, message, http.StatusInternalServerError)
}

func recordError(r *http.Request, err error, msg string) string {
	errID := newErrorID()
	span := trace.SpanFromContext(r.Context())
	span.RecordError(err,
		trace.WithAttributes(attribute.String("err_id", errID)),
		trace.WithAttributes(attribute.String("err_msg", msg)),
	)
	return errID
}

func newErrorID() string {
	// unique-enough, short, and unambigious, error reference for users to notify us
	return strings.ToUpper(hex.EncodeToString(securecookie.GenerateRandomKey(4)))
}
