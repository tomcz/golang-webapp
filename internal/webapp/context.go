package webapp

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/tomcz/gotools/maps"
)

type contextKey string

const (
	currentRequestIdKey = contextKey("current.requestId")
	currentMetadataKey  = contextKey("current.metadata")
	currentLoggerKey    = contextKey("current.logger")
)

type metadataFields struct {
	fields map[string]any
}

func (m *metadataFields) Set(key string, value any) {
	if value != nil {
		m.fields[key] = value
	}
}

func (m *metadataFields) Slice() []any {
	args := make([]any, 0, len(m.fields)*2)
	for k, v := range maps.SortedEntries(m.fields) {
		args = append(args, k, v)
	}
	return args
}

func newMetadataFields() *metadataFields {
	return &metadataFields{fields: make(map[string]any)}
}

func setupContext(r *http.Request, requestID string, log *slog.Logger, md *metadataFields) *http.Request {
	ctx := context.WithValue(r.Context(), currentRequestIdKey, requestID)
	ctx = context.WithValue(ctx, currentMetadataKey, md)
	ctx = context.WithValue(ctx, currentLoggerKey, log)
	return r.WithContext(ctx)
}

func RId(r *http.Request) string {
	if id, ok := r.Context().Value(currentRequestIdKey).(string); ok {
		return id
	}
	return ""
}

func RLog(r *http.Request) *slog.Logger {
	if log, ok := r.Context().Value(currentLoggerKey).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}

func RSet(r *http.Request, key string, value any) {
	if md, ok := r.Context().Value(currentMetadataKey).(*metadataFields); ok {
		md.Set(key, value)
	}
}
