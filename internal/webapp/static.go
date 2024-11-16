package webapp

import (
	"fmt"
	"net/http"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/static"
)

func registerStaticAssetRoutes(mux *http.ServeMux) {
	// Old-school cache-busting technique: add commit info so that we can use versioned
	// static paths to prevent browsers from using old assets with new deployments.
	prefix := fmt.Sprintf("/static/%s/", build.Commit())
	h := http.StripPrefix(prefix, http.FileServer(static.FS))
	mux.Handle("/static/", staticCacheControl(h))
}

func staticCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if build.IsProd {
			// embedded content can be cached by the browser for 10 minutes
			w.Header().Set("Cache-Control", "private, max-age=600")
		} else {
			// don't cache local assets, so we can work on them easily
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
