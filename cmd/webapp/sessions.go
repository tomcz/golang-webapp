package main

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	log "github.com/sirupsen/logrus"
)

const sessionKey = "session"

type sessionsStore struct {
	store sessions.Store
	name  string
}

func newSessionStore(authKey, encKey, sessionName string) *sessionsStore {
	return &sessionsStore{
		store: sessions.NewCookieStore(keyToBytes(authKey), keyToBytes(encKey)),
		name:  sessionName,
	}
}

func keyToBytes(key string) []byte {
	if key != "" {
		buf := []byte(key)
		if len(buf) == 32 {
			return buf
		}
		sum := sha256.Sum256(buf)
		return sum[:]
	}
	buf := securecookie.GenerateRandomKey(32)
	if buf == nil {
		panic("cannot generate random key")
	}
	return buf
}

func (s *sessionsStore) wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// We're ignoring the error resulted from decoding an existing session
		// since Get() always returns a session, even if empty.
		session, _ := s.store.Get(r, s.name)
		ctx := context.WithValue(r.Context(), sessionKey, session)
		fn(w, r.WithContext(ctx))
	}
}

func getSession(r *http.Request) *sessions.Session {
	if s, ok := r.Context().Value(sessionKey).(*sessions.Session); ok {
		return s
	}
	return nil
}

func saveSession(w http.ResponseWriter, r *http.Request) bool {
	s := getSession(r)
	if s == nil {
		return true // no session to save
	}
	err := s.Save(r, w)
	if err == nil {
		return true // saved properly
	}
	log.WithError(err).Error("failed to save session")
	http.Error(w, "failed to save session", http.StatusInternalServerError)
	return false
}

func redirect(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		http.Redirect(w, r, url, http.StatusFound)
	}
}
