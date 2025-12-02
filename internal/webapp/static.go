package webapp

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/tomcz/golang-webapp/static"
)

func registerStaticAssetRoutes(router *mux.Router, commit string) {
	// Old-school cache-busting technique: add commit info so that we can use versioned
	// static paths to prevent browsers from using old assets with new deployments.
	prefix := fmt.Sprintf("/static/%s/", commit)
	h := http.StripPrefix(prefix, http.FileServer(static.FS))
	router.PathPrefix("/static/").Handler(withStaticCacheControl(h)).Name("static")
}

func withStaticCacheControl(next http.Handler) http.Handler {
	// No goland, it isn't always false.
	//goland:noinspection GoBoolExpressions
	if static.Embedded {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// don't cache file assets so we can work on them easily
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
