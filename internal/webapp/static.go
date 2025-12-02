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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if static.Embedded {
			// embedded content can be cached by the browser for 10 minutes
			w.Header().Set("Cache-Control", "private, max-age=600")
		} else {
			// don't cache file assets so we can work on them easily
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
