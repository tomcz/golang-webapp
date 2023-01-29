package handlers

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

const (
	authUserKey   = "AuthUser"
	afterLoginKey = "AfterLogin"
)

func private(ss webapp.SessionStore, next http.HandlerFunc) http.HandlerFunc {
	return ss.Wrap(func(w http.ResponseWriter, r *http.Request) {
		s := webapp.CurrentSession(r)
		if user := s.GetString(authUserKey); user != "" {
			webapp.RSet(r, "auth_user", user)
			next(w, r)
			return
		}
		url := r.URL.Path
		query := r.URL.Query()
		if len(query) > 0 {
			url = fmt.Sprintf("%s?%s", url, query.Encode())
		}
		s.Set(afterLoginKey, url)
		webapp.RedirectTo(w, r, "showLogin")
	})
}

func showLogin(w http.ResponseWriter, r *http.Request) {
	webapp.Render(w, r, nil, "layout.gohtml", "login.gohtml")
}

func handleLogin(knownUsers map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := webapp.CurrentSession(r)
		username := strings.TrimSpace(r.PostFormValue("username"))
		password := strings.TrimSpace(r.PostFormValue("password"))
		expected, ok := knownUsers[username]
		if !ok {
			webapp.RSet(r, "err", fmt.Sprintf("unknown user %q", username))
			s.AddFlashError("Invalid credentials. Please try again.")
			webapp.RedirectTo(w, r, "showLogin")
			return
		}
		if subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 0 {
			webapp.RSet(r, "err", fmt.Sprintf("invalid password for user %q", username))
			s.AddFlashError("Invalid credentials. Please try again.")
			webapp.RedirectTo(w, r, "showLogin")
			return
		}
		s.Set(authUserKey, username)
		webapp.RSet(r, "auth_user", username)
		s.AddFlashMessage(fmt.Sprintf("Welcome %s!", username))
		if url := s.GetString(afterLoginKey); url != "" {
			webapp.Redirect(w, r, url)
			return
		}
		webapp.RedirectTo(w, r, "showIndex")
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	webapp.CurrentSession(r).Clear()
	webapp.RedirectTo(w, r, "showIndex")
}
