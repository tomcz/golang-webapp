package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const currentSessionKey = contextKey("current.session")

type sessionStore struct {
	store sessions.Store
	name  string
}

func newSessionStore(sessionName, authKey, encKey string) *sessionStore {
	return &sessionStore{
		store: sessions.NewCookieStore(keyToBytes(authKey), keyToBytes(encKey)),
		name:  sessionName,
	}
}

func keyToBytes(key string) []byte {
	if key != "" {
		buf, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			buf = []byte(key)
		}
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

func (s *sessionStore) wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// We're ignoring the error resulted from decoding an existing session
		// since Get() always returns a session, even if empty.
		session, _ := s.store.Get(r, s.name)
		ctx := context.WithValue(r.Context(), currentSessionKey, session)
		fn(w, r.WithContext(ctx))
	}
}

func getSession(r *http.Request) *sessions.Session {
	if s, ok := r.Context().Value(currentSessionKey).(*sessions.Session); ok {
		return s
	}
	return nil
}

func currentSession(r *http.Request) *sessions.Session {
	s := getSession(r)
	if s == nil {
		panic("no current session; is this handler wrapped?")
	}
	return s
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
	renderErr(w, r, fmt.Errorf("session save: %w", err), "Failed to save session")
	return false
}
