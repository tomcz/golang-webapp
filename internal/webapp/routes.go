package webapp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

const currentRouterKey = contextKey("current.router")

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(setRouter(r), setCurrentRouteName, noStoreCacheControl)
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

func setCurrentRouteName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			if name := route.GetName(); name != "" {
				RSet(r, "req_route", name)
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
		RenderErr(w, r, err, "cannot create redirect", http.StatusInternalServerError)
		return
	}
	url, err := router.Get(routeName).URL(pathVars...)
	if err != nil {
		RenderErr(w, r, err, "cannot create redirect", http.StatusInternalServerError)
		return
	}
	Redirect(w, r, url.String())
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		http.Redirect(w, r, url, http.StatusFound)
	}
}
