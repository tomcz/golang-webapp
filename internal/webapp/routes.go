package webapp

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tomcz/gotools/maps"
)

func NewRouter() *http.ServeMux {
	r := http.NewServeMux()
	registerStaticAssetRoutes(r)
	return r
}

func Register(r *http.ServeMux, pattern string, handler http.Handler) {
	r.Handle(pattern, noStoreCacheControl(handler))
}

func RegisterMethods(r *http.ServeMux, pattern string, methodRoutes map[string]http.HandlerFunc) {
	r.Handle(pattern, noStoreCacheControl(methodSwitchFunc(methodRoutes)))
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		// https://opentelemetry.io/docs/reference/specification/trace/semantic_conventions/http/#http-request-and-response-headers
		AddToSpan(r, "http.response.header.location", url)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

func noStoreCacheControl(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	}
}

func methodSwitchFunc(methodRoutes map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next, ok := methodRoutes[r.Method]
		if ok {
			next.ServeHTTP(w, r)
			return
		}
		allowed := strings.Join(maps.SortedKeys(methodRoutes), ", ")
		w.Header().Set("Allow", allowed)
		http.Error(w, fmt.Sprintf("Allowed: %s", allowed), http.StatusMethodNotAllowed)
	}
}
