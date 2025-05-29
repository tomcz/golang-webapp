package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()
	r.Handle("/{$}", "index", http.RedirectHandler("/index", http.StatusFound))

	r.HandleFunc("/error", "exampleError", exampleError)
	r.HandleFunc("/panic", "examplePanic", examplePanic)
	r.HandleFunc("/ping", "examplePing", examplePing)

	r.HandleFunc("GET /login", "showLogin", showLogin)
	r.HandleFunc("POST /login", "handleLogin", handleLogin(knownUsers))
	r.HandleFunc("/logout", "handleLogout", handleLogout)

	r.HandleFunc("GET /index", "showIndex", private(showIndex))
	r.HandleFunc("POST /index", "updateIndex", private(updateIndex))

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
