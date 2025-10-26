package webapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

const (
	currentRouterKey = contextKey("current.router")
	currentCommitKey = contextKey("current.commit")
)

type Router struct {
	router      *mux.Router
	store       sessions.Store
	sessionName string
	commit      string
	behindProxy bool
}

func NewRouter(store sessions.Store, sessionName string, commit string, behindProxy bool) *Router {
	router := mux.NewRouter()
	registerStaticAssetRoutes(router, commit)
	return &Router{
		router:      router,
		store:       store,
		sessionName: sessionName,
		commit:      commit,
		behindProxy: behindProxy,
	}
}

func (r *Router) Handle(path, name string, handler http.Handler) *mux.Route {
	return r.router.Handle(path, handler).Name(name)
}

func (r *Router) HandleFunc(path, name string, handler http.HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, handler).Name(name)
}

func (r *Router) HandleSession(path, name string, handler HandlerWithSession) *mux.Route {
	return r.HandleFunc(path, name, withSession(r.store, r.sessionName, handler))
}

func (r *Router) Handler() http.Handler {
	r.router.Use(
		withPanicRecovery,
		withCommit(r.commit),
		withRouter(r.router),
		withHandlerName,
		withDynamicCacheControl,
		withSecurityHeaders,
		withCsrfProtection,
	)
	handler := handlers.CustomLoggingHandler(io.Discard, r.router, logFormatter)
	if r.behindProxy {
		handler = handlers.ProxyHeaders(handler)
	}
	return withCurrentMetadata(handler)
}

func AbsoluteURL(r *http.Request, path string) string {
	var sb strings.Builder
	if r.URL.Scheme != "" {
		// set by handlers.ProxyHeaders
		sb.WriteString(r.URL.Scheme)
		sb.WriteString("://")
	} else if r.TLS != nil {
		// only present on https listeners
		sb.WriteString("https://")
	} else {
		sb.WriteString("http://")
	}
	// set by handlers.ProxyHeaders and net/http
	sb.WriteString(r.Host)
	sb.WriteString(path)
	return sb.String()
}

func PathFor(r *http.Request, routeName string, patternVars ...string) string {
	router, ok := r.Context().Value(currentRouterKey).(*mux.Router)
	if !ok {
		// this is a coding error that cannot be fixed at runtime
		panic("no current router")
	}
	routeURL, err := router.Get(routeName).URL(patternVars...)
	if err != nil {
		// this is a coding error that cannot be fixed at runtime
		panic(err)
	}
	path := routeURL.String()
	return path
}

func RedirectToRoute(w http.ResponseWriter, r *http.Request, routeName string, patternVars ...string) {
	RedirectToURL(w, r, PathFor(r, routeName, patternVars...))
}

func RedirectToURL(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		RSet(r, "res_location", url)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

func logFormatter(_ io.Writer, p handlers.LogFormatterParams) {
	reqDuration := time.Since(p.TimeStamp)
	fields := []any{
		"req_start_at", p.TimeStamp,
		"req_host", p.Request.Host,
		"req_method", p.Request.Method,
		"req_uri", p.Request.RequestURI,
		"req_user_agent", p.Request.UserAgent(),
		"req_remote_addr", p.Request.RemoteAddr,
		"res_status", p.StatusCode,
		"res_duration_ms", reqDuration.Milliseconds(),
		"res_duration_ns", reqDuration.Nanoseconds(),
		"res_size", p.Size,
	}
	md := currentMetadataFields(p.Request)
	for key, value := range md.fields {
		fields = append(fields, key, value)
	}
	if md.isDebug {
		md.logger.Debug("request finished", fields...)
	} else if p.StatusCode >= 500 {
		md.logger.Error("request finished", fields...)
	} else if p.StatusCode >= 400 && p.StatusCode != 404 {
		md.logger.Warn("request finished", fields...)
	} else {
		md.logger.Info("request finished", fields...)
	}
}

func withPanicRecovery(next http.Handler) http.Handler {
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

func withCommit(commit string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), currentCommitKey, commit)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func withRouter(router *mux.Router) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), currentRouterKey, router)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func withHandlerName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			RSet(r, "req_handler", route.GetName())
		}
		next.ServeHTTP(w, r)
	})
}

func withDynamicCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// Ref: https://blog.appcanary.com/2017/http-security-headers.html
// Use github.com/unrolled/secure when CSP, HSTS, or HPKP is needed.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-XSS-Protection", "1; mode=block")
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func withCsrfProtection(next http.Handler) http.Handler {
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
