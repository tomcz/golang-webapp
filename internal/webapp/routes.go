package webapp

import (
	"fmt"
	"net/http"
)

func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()
	registerStaticAssetRoutes(mux)
	return mux
}

func DynamicCacheControl(next http.Handler) http.Handler {
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

func RenderError(w http.ResponseWriter, r *http.Request, err error, msg string, statusCode int) {
	RSet(r, "error", err)
	msg = fmt.Sprintf("ID: %s\nError: %s\n", ReqID(r), msg)
	http.Error(w, msg, statusCode)
}
