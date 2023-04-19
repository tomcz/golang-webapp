package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore, knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()

	// no session
	webapp.Register(r, "/", webapp.WithNamedHandler("root", http.RedirectHandler("/index", http.StatusFound)))
	webapp.Register(r, "/error", webapp.WithNamedHandlerFunc("exampleError", exampleError))
	webapp.Register(r, "/panic", webapp.WithNamedHandlerFunc("examplePanic", examplePanic))

	// unauthenticated
	webapp.RegisterMethods(r, "/login", map[string]http.HandlerFunc{
		http.MethodGet:  public(s, "showLogin", showLogin),
		http.MethodPost: public(s, "handleLogin", handleLogin(knownUsers)),
	})
	webapp.Register(r, "/logout", public(s, "handleLogout", handleLogout))

	// authenticated
	webapp.RegisterMethods(r, "/index", map[string]http.HandlerFunc{
		http.MethodGet:  private(s, "showIndex", showIndex),
		http.MethodPost: private(s, "updateIndex", updateIndex),
	})

	return r
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	webapp.Redirect(w, r, "/login")
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	webapp.Redirect(w, r, "/index")
}
