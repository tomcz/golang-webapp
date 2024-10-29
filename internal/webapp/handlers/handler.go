package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore, knownUsers map[string]string) http.Handler {
	mux := http.NewServeMux()

	// no session
	mux.Handle("/", http.RedirectHandler("/index", http.StatusFound))
	mux.HandleFunc("/error", exampleError)
	mux.HandleFunc("/panic", examplePanic)

	// unauthenticated, with session
	mux.HandleFunc("GET /login", public(s, showLogin))
	mux.HandleFunc("POST /login", public(s, handleLogin(knownUsers)))
	mux.HandleFunc("/logout", public(s, handleLogout))

	// authenticated, session required
	mux.HandleFunc("GET /index", private(s, showIndex))
	mux.HandleFunc("POST /index", private(s, updateIndex))

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
