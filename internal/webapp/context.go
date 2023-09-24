package webapp

import (
	"context"
	"log/slog"
	"net/http"

	om "github.com/elliotchance/orderedmap/v2"
)

type contextKey string

const (
	currentRequestIdKey = contextKey("current.requestId")
	currentMetadataKey  = contextKey("current.metadata")
	currentLoggerKey    = contextKey("current.logger")
)

func setupContext(r *http.Request, requestID string, log *slog.Logger, md *om.OrderedMap[string, any]) *http.Request {
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

func rlog(r *http.Request) *slog.Logger {
	if log, ok := r.Context().Value(currentLoggerKey).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}

func RSet(r *http.Request, key string, value any) {
	if value == nil {
		return
	}
	if md, ok := r.Context().Value(currentMetadataKey).(*om.OrderedMap[string, any]); ok {
		md.Set(key, value)
	}
}
