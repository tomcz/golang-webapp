package webapp

import (
	"net/http"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/static"
)

func registerStaticAssetRoutes(r *http.ServeMux) {
	h := http.StripPrefix("/static/", http.FileServer(static.FS))
	r.Handle("/static/", staticCacheControl(WithHandlerName("static", h)))
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
