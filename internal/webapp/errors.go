package webapp

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
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
				errID := errorID()
				span := trace.SpanFromContext(r.Context())
				span.RecordError(err,
					trace.WithStackTrace(true),
					trace.WithAttributes(attribute.String("err_id", errID)),
					trace.WithAttributes(attribute.String("err_msg", "recovered from panic")),
				)
				errorLog(r, span, errID, err).Error("recovered from panic")
				msg := fmt.Sprintf("[%s] request failed", errID)
				http.Error(w, msg, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func RenderError(w http.ResponseWriter, r *http.Request, err error, msg string, statusCode int) {
	errID := RecordError(r, err, msg)
	message := fmt.Sprintf("[%s] %s", errID, msg)
	http.Error(w, message, statusCode)
}

func RecordError(r *http.Request, err error, msg string) string {
	errID := errorID()
	span := trace.SpanFromContext(r.Context())
	span.RecordError(err,
		trace.WithAttributes(attribute.String("err_id", errID)),
		trace.WithAttributes(attribute.String("err_msg", msg)),
	)
	errorLog(r, span, errID, err).Warn(msg)
	return errID
}

func errorLog(r *http.Request, span trace.Span, errID string, err error) logrus.FieldLogger {
	ctx := span.SpanContext()
	return getLogger(r).WithError(err).
		WithField("err_id", errID).
		WithField("req_method", r.Method).
		WithField("req_path", r.URL.Path).
		WithField("trace_id", ctx.TraceID()).
		WithField("span_id", ctx.SpanID())
}

// unique-enough, short, and unambiguous, error reference for users to notify us
func errorID() string {
	buf, err := randomBytes(4)
	if err != nil {
		return "XXXX"
	}
	id := hex.EncodeToString(buf)
	return strings.ToUpper(id)
}
