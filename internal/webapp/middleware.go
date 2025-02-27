package webapp

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/streadway/handy/breaker"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
)

func WithMiddleware(h http.Handler, withTLS bool) http.Handler {
	sm := secure.New(secure.Options{
		BrowserXssFilter:     true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		ReferrerPolicy:       "no-referrer",
		SSLRedirect:          false,
		SSLTemporaryRedirect: false,
		IsDevelopment:        !withTLS, // don't enable production settings without TLS
	})
	h = sm.Handler(h)
	h = panicRecovery(h)
	h = breaker.Handler(breaker.NewBreaker(0.1), breaker.DefaultStatusCodeValidator, h)
	return requestLogger(h)
}

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				stack := string(debug.Stack())
				RSet(r, "panic_stack", stack)
				err := fmt.Errorf("panic: %v", p)
				HttpError(w, r, http.StatusInternalServerError, "Request failed", err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		fields := newMetadataFields()
		fields.Set("req_start_at", start)
		fields.Set("req_host", r.Host)
		fields.Set("req_method", r.Method)
		fields.Set("req_path", r.URL.Path)
		fields.Set("req_user_agent", r.UserAgent())
		fields.Set("req_remote_addr", RemoteAddr(r))
		if referer := r.Referer(); referer != "" {
			fields.Set("req_referer", referer)
		}

		rr := setupContext(r, fields)
		ww := negroni.NewResponseWriter(w)

		next.ServeHTTP(ww, rr)

		duration := time.Since(start)
		fields.Set("res_status", ww.Status())
		fields.Set("res_duration_ms", duration.Milliseconds())
		fields.Set("res_duration_ns", duration.Nanoseconds())
		fields.Set("res_size", ww.Size())
		if loc := ww.Header().Get("Location"); loc != "" {
			fields.Set("res_location", loc)
		}

		args := fields.Slice()
		if fields.isDebug {
			fields.logger.Debug("request finished", args...)
		} else {
			fields.logger.Info("request finished", args...)
		}
	})
}
