package webapp

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/tomcz/gotools/maps"
)

type contextKey string

const currentMetadataKey = contextKey("current.metadata")

type metadataFields struct {
	fields    map[string]any
	logger    *slog.Logger
	requestID string
	isDebug   bool
}

func (m *metadataFields) Set(key string, value any) {
	if value != nil {
		m.fields[key] = value
	}
}

func (m *metadataFields) Slice() []any {
	args := make([]any, 0, len(m.fields)*2)
	for _, e := range maps.SortedEntries(m.fields) {
		args = append(args, e.Key, e.Val)
	}
	return args
}

func newMetadataFields(requestID string, log *slog.Logger) *metadataFields {
	return &metadataFields{
		fields:    make(map[string]any),
		logger:    log,
		requestID: requestID,
	}
}

func setupContext(r *http.Request, md *metadataFields) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), currentMetadataKey, md))
}

func RId(r *http.Request) string {
	if md, ok := r.Context().Value(currentMetadataKey).(*metadataFields); ok {
		return md.requestID
	}
	return "XXX"
}

func RLog(r *http.Request) *slog.Logger {
	if md, ok := r.Context().Value(currentMetadataKey).(*metadataFields); ok {
		return md.logger
	}
	return slog.Default()
}

func RSet(r *http.Request, key string, value any) {
	if md, ok := r.Context().Value(currentMetadataKey).(*metadataFields); ok {
		md.Set(key, value)
	}
}

// RDebug sets this request to be logged at DEBUG level rather than the INFO default.
// Useful, for example, for health check endpoints so that they don't flood production logs.
func RDebug(r *http.Request) {
	if md, ok := r.Context().Value(currentMetadataKey).(*metadataFields); ok {
		md.isDebug = true
	}
}
