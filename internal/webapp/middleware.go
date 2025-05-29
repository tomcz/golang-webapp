package webapp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"time"

	"github.com/gorilla/sessions"
	"github.com/streadway/handy/breaker"
	"github.com/urfave/negroni"
)

func WithMiddleware(store sessions.Store, sessionName string, h http.Handler) http.Handler {
	h = panicRecovery(securityHeaders(csrfProtection(sessionMiddleware(store, sessionName, h))))
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
		status := ww.Status()

		fields.Set("res_status", status)
		fields.Set("res_duration_ms", duration.Milliseconds())
		fields.Set("res_duration_ns", duration.Nanoseconds())
		fields.Set("res_size", ww.Size())
		if loc := ww.Header().Get("Location"); loc != "" {
			fields.Set("res_location", loc)
		}

		args := fields.Slice()
		if fields.isDebug {
			fields.logger.Debug("request finished", args...)
		} else if status >= 500 {
			fields.logger.Error("request finished", args...)
		} else if status >= 400 && status != 404 {
			fields.logger.Warn("request finished", args...)
		} else {
			fields.logger.Info("request finished", args...)
		}
	})
}

// Ref: https://blog.appcanary.com/2017/http-security-headers.html
// Use github.com/unrolled/secure when CSP, HSTS, or HPKP is needed.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-XSS-Protection", "1; mode=block")
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := csrfCheck(r); err != nil {
			HttpError(w, r, http.StatusBadRequest, "CSRF validation failed", err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var csrfSafeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

var csrfSafeFetches = map[string]bool{
	"same-origin": true,
	"none":        true,
}

// adapted from https://github.com/golang/go/issues/73626
func csrfCheck(r *http.Request) error {
	if csrfSafeMethods[r.Method] {
		return nil
	}
	secFetchSite := r.Header.Get("Sec-Fetch-Site")
	if csrfSafeFetches[secFetchSite] {
		return nil
	}
	origin := r.Header.Get("Origin")
	if secFetchSite == "" {
		if origin == "" {
			// we could also fail open here and allow requests from curl, etc.
			return errors.New("not a browser request")
		}
		parsed, err := url.Parse(origin)
		if err != nil {
			return fmt.Errorf("bad origin: %w", err)
		}
		if parsed.Host == r.Host {
			return nil
		}
	}
	return fmt.Errorf("Sec-Fetch-Site %q, Origin %q, Host %q", secFetchSite, origin, r.Host)
}
