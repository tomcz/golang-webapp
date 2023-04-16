package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore, knownUsers map[string]string) http.Handler {
	r := webapp.NewRouter()

	// no session
	webapp.Register(r, "/", webapp.WithHandlerName("root", http.RedirectHandler("/index", http.StatusFound)))
	webapp.Register(r, "/error", webapp.WithHandlerName("exampleError", http.HandlerFunc(exampleError)))
	webapp.Register(r, "/panic", webapp.WithHandlerName("examplePanic", http.HandlerFunc(examplePanic)))

	// unauthenticated
	webapp.RegisterMethods(r, "/login", map[string]http.HandlerFunc{
		"GET":  public(s, "showLogin", showLogin),
		"POST": public(s, "handleLogin", handleLogin(knownUsers)),
	})
	webapp.Register(r, "/logout", public(s, "handleLogout", handleLogout))

	// authenticated
	webapp.RegisterMethods(r, "/index", map[string]http.HandlerFunc{
		"GET":  private(s, "showIndex", showIndex),
		"POST": private(s, "updateIndex", updateIndex),
	})

	return r
}
