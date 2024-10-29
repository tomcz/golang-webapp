package webapp

import "net/http"

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
