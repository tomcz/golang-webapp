package app

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

type HandlerConfig struct {
	Sessions    webapp.SessionProvider
	KnownUsers  map[string]string
	Commit      string
	BehindProxy bool
}

func Handler(cfg HandlerConfig) http.Handler {
	r := webapp.NewRouter(cfg.Sessions, cfg.BehindProxy, cfg.Commit)

	r.HandleFunc("/", "root", redirectToIndex)

	r.HandleSession("/login", "showLogin", showLogin).Methods("GET")
	r.HandleSession("/login", "handleLogin", handleLogin(cfg.KnownUsers)).Methods("POST")
	r.HandleSession("/logout", "handleLogout", handleLogout)

	r.HandleSession("/index", "showIndex", private(showIndex)).Methods("GET")
	r.HandleSession("/index", "updateIndex", private(updateIndex)).Methods("POST")

	return r.Handler()
}

func isPartial(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectToRoute(w, r, "showLogin")
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	webapp.RedirectToRoute(w, r, "showIndex")
}
