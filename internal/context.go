package internal

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
)

type contextKey string

const (
	currentRequestIdKey = contextKey("current.requestId")
	currentMetadataKey  = contextKey("current.metadata")
)

func setup(r *http.Request, requestID string, md logrus.Fields) *http.Request {
	ctx := context.WithValue(r.Context(), currentRequestIdKey, requestID)
	ctx = context.WithValue(ctx, currentMetadataKey, md)
	return r.WithContext(ctx)
}

func rid(r *http.Request) string {
	if id, ok := r.Context().Value(currentRequestIdKey).(string); ok {
		return id
	}
	return ""
}

func rset(r *http.Request, key string, value any) {
	if md, ok := r.Context().Value(currentMetadataKey).(logrus.Fields); ok {
		md[key] = value
	}
}
