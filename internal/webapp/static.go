package webapp

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/static"
)

func registerStaticAssetRoutes(r *mux.Router) {
	// add commit info so we can set versioned static paths
	// to prevent browsers using old assets with a new version
	prefix := fmt.Sprintf("/static/%s/", build.Commit())
	h := http.StripPrefix(prefix, http.FileServer(static.FS))
	r.PathPrefix("/static/").Handler(staticCacheControl(h)).Name("static")
}

func staticCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if build.IsProd {
			// embedded content can be cached by the browser for 10 minutes
			w.Header().Set("Cache-Control", "private, max-age=600")
		} else {
			// don't cache local assets, so we can work on them easily
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}
