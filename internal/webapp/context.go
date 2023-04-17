package webapp

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
)

type contextKey string

const (
	currentRequestIdKey = contextKey("current.requestId")
	currentMetadataKey  = contextKey("current.metadata")
	currentLoggerKey    = contextKey("current.logger")
)

func setupContext(r *http.Request, requestID string, log logrus.FieldLogger, md logrus.Fields) *http.Request {
	ctx := context.WithValue(r.Context(), currentRequestIdKey, requestID)
	ctx = context.WithValue(ctx, currentMetadataKey, md)
	ctx = context.WithValue(ctx, currentLoggerKey, log)
	return r.WithContext(ctx)
}

func rid(r *http.Request) string {
	if id, ok := r.Context().Value(currentRequestIdKey).(string); ok {
		return id
	}
	return ""
}

func rlog(r *http.Request) logrus.FieldLogger {
	if log, ok := r.Context().Value(currentLoggerKey).(logrus.FieldLogger); ok {
		return log
	}
	return logrus.New()
}

func RSet(r *http.Request, key string, value any) {
	if value == nil {
		return
	}
	if md, ok := r.Context().Value(currentMetadataKey).(logrus.Fields); ok {
		md[key] = value
	}
}
