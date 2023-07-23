package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore, knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()

	// no session
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound)).Name("root")
	r.HandleFunc("/error", exampleError).Name("exampleError")
	r.HandleFunc("/panic", examplePanic).Name("examplePanic")

	// unauthenticated, with session
	r.HandleFunc("/login", public(s, showLogin)).Name("showLogin").Methods(http.MethodGet)
	r.HandleFunc("/login", public(s, handleLogin(knownUsers))).Name("handleLogin").Methods(http.MethodPost)
	r.HandleFunc("/logout", public(s, handleLogout)).Name("handleLogout")

	// authenticated, session required
	r.HandleFunc("/index", private(s, showIndex)).Name("showIndex").Methods(http.MethodGet)
	r.HandleFunc("/index", private(s, updateIndex)).Name("updateIndex").Methods(http.MethodPost)

	return r
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectTo(w, r, "showLogin")
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectTo(w, r, "showIndex")
}
