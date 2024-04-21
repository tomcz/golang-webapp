package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore, knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()

	// no session
	r.Handle("root", "/", http.RedirectHandler("/index", http.StatusFound))
	r.HandleFunc("exampleError", "/error", exampleError)
	r.HandleFunc("examplePanic", "/panic", examplePanic)

	// unauthenticated, with session
	r.HandleFunc("showLogin", "GET /login", public(s, showLogin))
	r.HandleFunc("handleLogin", "POST /login", public(s, handleLogin(knownUsers)))
	r.HandleFunc("handleLogout", "/logout", public(s, handleLogout))

	// authenticated, session required
	r.HandleFunc("showIndex", "GET /index", private(s, showIndex))
	r.HandleFunc("updateIndex", "POST /index", private(s, updateIndex))

	return r.Handler()
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectTo(w, r, "/login")
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectTo(w, r, "/index")
}
