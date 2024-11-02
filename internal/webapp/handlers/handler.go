package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(sw webapp.SessionWrapper, knownUsers map[string]string) http.Handler {
	mux := http.NewServeMux()

	// no session
	mux.Handle("/{$}", http.RedirectHandler("/index", http.StatusFound))
	mux.HandleFunc("/error", exampleError)
	mux.HandleFunc("/panic", examplePanic)
	mux.HandleFunc("/ping", examplePing)

	// unauthenticated, with session
	mux.HandleFunc("GET /login", public(sw, showLogin))
	mux.HandleFunc("POST /login", public(sw, handleLogin(knownUsers)))
	mux.HandleFunc("/logout", public(sw, handleLogout))

	// authenticated, session required
	mux.HandleFunc("GET /index", private(sw, showIndex))
	mux.HandleFunc("POST /index", private(sw, updateIndex))

	webapp.RegisterStaticAssetRoutes(mux)
	return webapp.DynamicCacheControl(mux)
}

func isPartial(r *http.Request) bool {
	return r.Header.Get("Hx-Request") == "true"
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectToUrl(w, r, "/login")
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectToUrl(w, r, "/index")
}
