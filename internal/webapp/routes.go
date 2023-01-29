package webapp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

const currentRouterKey = contextKey("current.router")

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(setRouter(r), setCurrentRouteAttributes, noStoreCacheControl)
	registerStaticAssetRoutes(r)
	return r
}

func setRouter(router *mux.Router) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), currentRouterKey, router)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setCurrentRouteAttributes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			span := trace.SpanFromContext(r.Context())
			if name := route.GetName(); name != "" {
				// Technically-speaking this should be the URL http server name,
				// but otelhttp uses the name of the operation (i.e. "handler"),
				// so let's set it to the name of the matched gorilla/mux route.
				span.SetAttributes(semconv.HTTPServerNameKey.String(name))
			}
			if tmpl, err := route.GetPathTemplate(); err == nil {
				// These aren't in the spec format of /path/:id, but since we're
				// matching with gorilla/mux we can only provide what we have.
				span.SetAttributes(semconv.HTTPRouteKey.String(tmpl))
			} else {
				// No template found, we can just use the path as the route
				// since we want this field to be present for all requests.
				span.SetAttributes(semconv.HTTPRouteKey.String(r.URL.Path))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func noStoreCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func RedirectTo(w http.ResponseWriter, r *http.Request, routeName string, pathVars ...string) {
	router, ok := r.Context().Value(currentRouterKey).(*mux.Router)
	if !ok {
		err := fmt.Errorf("%s not in context", currentRouterKey)
		RenderError(w, r, err, "cannot create redirect", http.StatusInternalServerError)
		return
	}
	url, err := router.Get(routeName).URL(pathVars...)
	if err != nil {
		RenderError(w, r, err, "cannot create redirect", http.StatusInternalServerError)
		return
	}
	Redirect(w, r, url.String())
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		// https://opentelemetry.io/docs/reference/specification/trace/semantic_conventions/http/#http-request-and-response-headers
		AddToSpan(r, "http.response.header.location", url)
		http.Redirect(w, r, url, http.StatusFound)
	}
}
