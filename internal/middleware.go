package internal

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// prevent conflict with other
// similarly-named context keys
type contextKey string

const loggerKey = contextKey("request.logger")

func WithMiddleware(h http.Handler, withTLS bool, log logrus.FieldLogger) http.Handler {
	sm := secure.New(secure.Options{
		BrowserXssFilter:     true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		ReferrerPolicy:       "no-referrer",
		SSLRedirect:          true,
		SSLTemporaryRedirect: true,
		IsDevelopment:        withTLS, // don't enable production settings without TLS
	})
	h = sm.Handler(h)
	h = setLogger(panicRecovery(h), log.WithField("component", "middleware"))
	h = breaker.Handler(breaker.NewBreaker(0.1), breaker.DefaultStatusCodeValidator, h)
	// We want to trace every request, whether matched by handler.go or not,
	// and we want to capture all panics and circuit breaker actions, so we're
	// using the otelhttp handler and not otel's gorilla/mux middleware.
	return otelhttp.NewHandler(h, "handler")
}

func setLogger(next http.Handler, log logrus.FieldLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), loggerKey, log)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getLogger(r *http.Request) logrus.FieldLogger {
	if log, ok := r.Context().Value(loggerKey).(logrus.FieldLogger); ok {
		return log
	}
	return logrus.New()
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
			} else {
				// No template found, we can just use the path as the route
				// since we want this field to be present for all requests.
				span.SetAttributes(semconv.HTTPRouteKey.String(r.URL.Path))
			}
		}
		next.ServeHTTP(w, r)
	})
}
