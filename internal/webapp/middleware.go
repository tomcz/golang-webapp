package webapp

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
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
		IsDevelopment:        !withTLS, // don't enable production settings without TLS
	})
	h = sm.Handler(h)
	h = panicRecovery(h)
	h = setLogger(h, log.WithField("component", "middleware"))
	h = breaker.Handler(breaker.NewBreaker(0.1), breaker.DefaultStatusCodeValidator, h)
	// We want to trace every request, whether matched by handler.go or not,
	// and we want to capture all panics and circuit breaker actions, so we're
	// using the otelhttp handler and not otel's gorilla/mux middleware.
	return otelhttp.NewHandler(h, "handler")
}

func AddToSpan(r *http.Request, key, value string) {
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String(key, value))
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
