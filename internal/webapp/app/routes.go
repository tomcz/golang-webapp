package app

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func RegisterRoutes(r *webapp.Router, knownUsers map[string]string) {
	r.HandleFunc("/", "root", redirectToIndex)

	r.HandleSession("/login", "showLogin", showLogin).Methods("GET")
	r.HandleSession("/login", "handleLogin", handleLogin(knownUsers)).Methods("POST")
	r.HandleSession("/logout", "handleLogout", handleLogout)

	r.HandleSession("/index", "showIndex", private(showIndex)).Methods("GET")
	r.HandleSession("/index", "updateIndex", private(updateIndex)).Methods("POST")
}

func isPartial(r *http.Request) bool {
	return r.Header.Get("Hx-Request") == "true"
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectToRoute(w, r, "showLogin")
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectToRoute(w, r, "showIndex")
}
