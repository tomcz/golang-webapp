package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	return requestLogger(h)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("").Start(r.Context(), "http_request")
		defer span.End()

		rw := negroni.NewResponseWriter(w)
		next.ServeHTTP(rw, r.WithContext(ctx))

		statusCode := rw.Status()
		span.SetAttributes(
			attribute.Int("res_status", statusCode),
			attribute.Int("res_size", rw.Size()),
			attribute.String("req_host", r.Host),
			attribute.String("req_method", r.Method),
			attribute.String("req_url_path", r.URL.Path),
			attribute.String("req_user_agent", r.UserAgent()),
			attribute.String("req_remote_addr", r.RemoteAddr),
		)
		if referer := r.Referer(); referer != "" {
			span.SetAttributes(attribute.String("req_referer", referer))
		}
		if loc := rw.Header().Get("Location"); loc != "" {
			span.SetAttributes(attribute.String("res_location", loc))
		}
		if statusCode < 400 {
			span.SetStatus(codes.Ok, http.StatusText(statusCode))
		} else {
			span.SetStatus(codes.Error, http.StatusText(statusCode))
		}
	})
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
				errID := errorID()
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

func renderError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	errID := recordError(r, err, msg)
	message := fmt.Sprintf("[%s] %s", errID, msg)
	http.Error(w, message, http.StatusInternalServerError)
}

func recordError(r *http.Request, err error, msg string) string {
	errID := errorID()
	span := trace.SpanFromContext(r.Context())
	span.RecordError(err,
		trace.WithAttributes(attribute.String("err_id", errID)),
		trace.WithAttributes(attribute.String("err_msg", msg)),
	)
	return errID
}

func errorID() string {
	// unique-enough, short, and unambigious, error reference for users to notify us
	return strings.ToUpper(hex.EncodeToString(securecookie.GenerateRandomKey(4)))
}
