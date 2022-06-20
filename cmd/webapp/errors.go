package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/securecookie"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				var err error
				if e, ok := p.(error); ok {
					err = fmt.Errorf("panic: %w", e)
				} else {
					err = fmt.Errorf("panic: %v", p)
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
