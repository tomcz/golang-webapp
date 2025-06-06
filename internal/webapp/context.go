package webapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"slices"
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
	var keys []string
	for key := range m.fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var args []any
	for _, key := range keys {
		args = append(args, key, m.fields[key])
	}
	return args
}

func newMetadataFields() *metadataFields {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	reqID := hex.EncodeToString(buf)
	return &metadataFields{
		fields:    make(map[string]any),
		logger:    slog.With("component", "web", "req_id", reqID),
		requestID: reqID,
	}
}

func setupContext(r *http.Request, md *metadataFields) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), currentMetadataKey, md))
}

func currentMetadataFields(r *http.Request) (*metadataFields, bool) {
	md, ok := r.Context().Value(currentMetadataKey).(*metadataFields)
	return md, ok
}

func ReqID(r *http.Request) string {
	if md, ok := currentMetadataFields(r); ok {
		return md.requestID
	}
	return "XXX"
}

func RLog(r *http.Request) *slog.Logger {
	if md, ok := currentMetadataFields(r); ok {
		return md.logger
	}
	return slog.Default()
}

func RSet(r *http.Request, key string, value any) {
	if md, ok := currentMetadataFields(r); ok {
		md.Set(key, value)
	}
}

// RDebug sets this request to be logged at DEBUG level rather than the INFO default.
// Useful, for example, in health check endpoints so that they don't flood production logs.
func RDebug(r *http.Request) {
	if md, ok := currentMetadataFields(r); ok {
		md.isDebug = true
	}
}

func RemoteAddr(r *http.Request) string {
	// TODO: add more places to lookup remote address if sitting behind proxies (e.g. X-Forwarded-For)
	return r.RemoteAddr
}
