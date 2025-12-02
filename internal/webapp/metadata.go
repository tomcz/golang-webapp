package webapp

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
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

func newMetadataFields() *metadataFields {
	reqID := rand.Text()
	return &metadataFields{
		fields:    make(map[string]any),
		logger:    slog.With("component", "web", "req_id", reqID),
		requestID: reqID,
	}
}

func withCurrentMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), currentMetadataKey, newMetadataFields())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentMetadataFields(r *http.Request) *metadataFields {
	return r.Context().Value(currentMetadataKey).(*metadataFields)
}

func ReqID(r *http.Request) string {
	md := currentMetadataFields(r)
	return md.requestID
}

func RLog(r *http.Request) *slog.Logger {
	md := currentMetadataFields(r)
	return md.logger
}

func RSet(r *http.Request, key string, value any) {
	md := currentMetadataFields(r)
	md.Set(key, value)
}

// RDebug sets this request to be logged at DEBUG level rather than the INFO default.
// Useful, for example, in health check endpoints so that they don't flood production logs.
func RDebug(r *http.Request) {
	md := currentMetadataFields(r)
	md.isDebug = true
}
