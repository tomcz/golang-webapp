package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore, knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()

	// no session
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound)).Name("root")
	r.HandleFunc("/error", exampleError).Methods("GET").Name("exampleError")
	r.HandleFunc("/panic", examplePanic).Methods("GET").Name("examplePanic")

	// unauthenticated
	r.HandleFunc("/login", s.Wrap(showLogin)).Methods("GET").Name("showLogin")
	r.HandleFunc("/login", s.Wrap(handleLogin(knownUsers))).Methods("POST").Name("handleLogin")
	r.HandleFunc("/logout", s.Wrap(handleLogout)).Methods("GET", "POST").Name("handleLogout")

	// authenticated
	r.HandleFunc("/index", private(s, showIndex)).Methods("GET").Name("showIndex")
	r.HandleFunc("/index", private(s, updateIndex)).Methods("POST").Name("updateIndex")

	return r
}
