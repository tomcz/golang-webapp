package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(sw webapp.SessionWrapper, knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()

	// no session
	r.Handle("/{$}", "index", http.RedirectHandler("/index", http.StatusFound))
	r.HandleFunc("/error", "exampleError", exampleError)
	r.HandleFunc("/panic", "examplePanic", examplePanic)
	r.HandleFunc("/ping", "examplePing", examplePing)

	// unauthenticated, with session
	r.HandleFunc("GET /login", "showLogin", public(sw, showLogin))
	r.HandleFunc("POST /login", "handleLogin", public(sw, handleLogin(knownUsers)))
	r.HandleFunc("/logout", "handleLogout", public(sw, handleLogout))

	// authenticated, session required
	r.HandleFunc("GET /index", "showIndex", private(sw, showIndex))
	r.HandleFunc("POST /index", "updateIndex", private(sw, updateIndex))

	return r.Handler()
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
