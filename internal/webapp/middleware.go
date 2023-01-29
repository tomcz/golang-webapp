package webapp

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
)

func WithMiddleware(h http.Handler, log logrus.FieldLogger, withTLS bool) http.Handler {
	sm := secure.New(secure.Options{
		BrowserXssFilter:     true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		ReferrerPolicy:       "no-referrer",
		SSLRedirect:          true,
		SSLTemporaryRedirect: true,
		IsDevelopment:        !withTLS, // don't enable production settings without TLS
	})
	h = sm.Handler(h)
	h = panicRecovery(h)
	h = breaker.Handler(breaker.NewBreaker(0.1), breaker.DefaultStatusCodeValidator, h)
	return requestLogger(h, log.WithField("component", "middleware"))
}

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				stack := string(debug.Stack())
				RSet(r, "panic_stack", stack)
				err := fmt.Errorf("panic: %v", p)
				RenderErr(w, r, err, "Request failed", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler, log logrus.FieldLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		fields := logrus.Fields{}
		reqID := uuid.New().String()
		rr := setupContext(r, reqID, fields)
		ww := negroni.NewResponseWriter(w)

		next.ServeHTTP(ww, rr)

		duration := time.Since(start)

		fields["req_id"] = reqID
		fields["req_start_at"] = start
		fields["res_duration_ms"] = duration.Milliseconds()
		fields["res_duration_ns"] = duration.Nanoseconds()
		fields["res_status"] = ww.Status()
		fields["res_size"] = ww.Size()
		fields["req_host"] = r.Host
		fields["req_method"] = r.Method
		fields["req_path"] = r.URL.Path
		fields["req_user_agent"] = r.UserAgent()
		fields["req_remote_addr"] = r.RemoteAddr

		if referer := r.Referer(); referer != "" {
			fields["req_referer"] = referer
		}
		if loc := ww.Header().Get("Location"); loc != "" {
			fields["res_location"] = loc
		}
		log.WithFields(fields).Info("request finished")
	})
}
