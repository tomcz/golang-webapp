package app

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

func private(next webapp.HandlerWithSession) webapp.HandlerWithSession {
	return func(w http.ResponseWriter, r *http.Request, s webapp.Session) {
		if user := s.GetString(authUserKey); user != "" {
			webapp.RSet(r, "auth_user", user)
			next(w, r, s)
			return
		}
		if isHtmx(r) {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.Set(afterLoginKey, r.RequestURI)
		redirectToLogin(w, r)
	}
}

func authData(s webapp.Session) map[string]any {
	return map[string]any{authUserKey: s.GetString(authUserKey)}
}

func showLogin(w http.ResponseWriter, r *http.Request, s webapp.Session) {
	if user := s.GetString(authUserKey); user != "" {
		redirectToIndex(w, r)
		return
	}
	webapp.Render(w, r, "login.gohtml", nil)
}

func handleLogin(knownUsers map[string]string) webapp.HandlerWithSession {
	return func(w http.ResponseWriter, r *http.Request, s webapp.Session) {
		username := strings.TrimSpace(r.PostFormValue("username"))
		password := strings.TrimSpace(r.PostFormValue("password"))
		expected, ok := knownUsers[username]
		if !ok {
			webapp.RSet(r, "auth_error", fmt.Sprintf("unknown user %q", username))
			s.AddFlashError("Invalid credentials. Please try again.")
			redirectToLogin(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 0 {
			webapp.RSet(r, "auth_error", fmt.Sprintf("invalid password for user %q", username))
			s.AddFlashError("Invalid credentials. Please try again.")
			redirectToLogin(w, r)
			return
		}
		s.Set(authUserKey, username)
		webapp.RSet(r, "auth_user", username)
		if redirect := s.GetString(afterLoginKey); redirect != "" {
			webapp.RedirectToURL(w, r, redirect)
			return
		}
		redirectToIndex(w, r)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request, s webapp.Session) {
	s.Clear()
	redirectToIndex(w, r)
}
