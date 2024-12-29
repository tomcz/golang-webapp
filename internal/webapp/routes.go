package webapp

import "net/http"

type Router struct {
	mux *http.ServeMux
}

func NewRouter() *Router {
	mux := http.NewServeMux()
	registerStaticAssetRoutes(mux)
	return &Router{mux}
}

func (r *Router) Handle(pattern, name string, handler http.Handler) {
	r.mux.Handle(pattern, withHandlerName(name, handler))
}

func (r *Router) HandleFunc(pattern, name string, handler http.HandlerFunc) {
	r.mux.Handle(pattern, withHandlerName(name, handler))
}

func (r *Router) Handler() http.Handler {
	return withDynamicCacheControl(r.mux)
}

func withHandlerName(name string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RSet(r, "req_handler", name)
		next.ServeHTTP(w, r)
	})
}

func withDynamicCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func RedirectToUrl(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		http.Redirect(w, r, url, http.StatusFound)
	}
}
