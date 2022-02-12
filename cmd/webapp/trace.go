package main

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const traceKey = "request_id"

func traceRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), traceKey, uuid.New().String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func rlog(r *http.Request) log.FieldLogger {
	if id, ok := r.Context().Value(traceKey).(string); ok {
		return log.WithField(traceKey, id)
	}
	return log.New()
}

func rmsg(r *http.Request, msg string) string {
	if id, ok := r.Context().Value(traceKey).(string); ok {
		return id + ": " + msg
	}
	return msg
}
