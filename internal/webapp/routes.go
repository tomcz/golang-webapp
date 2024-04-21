package webapp

import (
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

type Router struct {
	mux *http.ServeMux
}

func NewRouter() *Router {
	r := &Router{mux: http.NewServeMux()}
	registerStaticAssetRoutes(r)
	return r
}

func (r *Router) Handle(name, pattern string, handler http.Handler) {
	r.mux.Handle(pattern, setCurrentRouteAttributes(name, pattern, handler))
}

func (r *Router) HandleFunc(name, pattern string, handler http.HandlerFunc) {
	r.mux.Handle(pattern, setCurrentRouteAttributes(name, pattern, handler))
}

func (r *Router) Handler() http.Handler {
	return noStoreCacheControl(r.mux)
}

func RedirectTo(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		// https://opentelemetry.io/docs/reference/specification/trace/semantic_conventions/http/#http-request-and-response-headers
		AddToSpan(r, "http.response.header.location", url)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

func setCurrentRouteAttributes(name, pattern string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		// otelhttp uses the same name (i.e. "handler") but let's not
		span.SetAttributes(semconv.HTTPServerNameKey.String(name))
		span.SetAttributes(semconv.HTTPRouteKey.String(pattern))
		next.ServeHTTP(w, r)
	})
}

func noStoreCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
