package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

const currentRouterKey = contextKey("current.router")

func setRouter(router *mux.Router) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), currentRouterKey, router)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func redirectToRoute(w http.ResponseWriter, r *http.Request, routeName string, pathVars ...string) {
	router, ok := r.Context().Value(currentRouterKey).(*mux.Router)
	if !ok {
		err := fmt.Errorf("%s not in context", currentRouterKey)
		renderErr(w, r, err, "cannot create redirect")
		return
	}
	url, err := router.Get(routeName).URL(pathVars...)
	if err != nil {
		renderErr(w, r, err, "cannot create redirect")
		return
	}
	redirect(w, r, url.String())
}

func redirect(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		http.Redirect(w, r, url, http.StatusFound)
	}
}
