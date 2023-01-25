package webapp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const currentSessionKey = contextKey("current.session")

type SessionStore interface {
	Wrap(fn http.HandlerFunc) http.HandlerFunc
}

type Session interface {
	Set(key string, value any)
	SetString(key string, value string)
	Get(key string) (any, bool)
	GetString(key string) (string, bool)
}

type sessionStore struct {
	store sessions.Store
	name  string
}

type currentSession struct {
	session *sessions.Session
}

func NewSessionStore(sessionName, authKey, encKey string) SessionStore {
	return &sessionStore{
		store: sessions.NewCookieStore(keyToBytes(authKey), keyToBytes(encKey)),
		name:  sessionName,
	}
}

func (s *sessionStore) Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// We're ignoring the error resulted from decoding an existing session
		// since Get() always returns a session, even if empty.
		session, _ := s.store.Get(r, s.name)
		ctx := context.WithValue(r.Context(), currentSessionKey, session)
		fn(w, r.WithContext(ctx))
	}
}

func CurrentSession(r *http.Request) Session {
	s := getSession(r)
	if s == nil {
		panic("no current session; is this handler wrapped?")
	}
	return &currentSession{s}
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

func getSession(r *http.Request) *sessions.Session {
	if s, ok := r.Context().Value(currentSessionKey).(*sessions.Session); ok {
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
	RenderError(w, r, err, "failed to save session")
	return false
}

func (c *currentSession) Set(key string, value any) {
	c.session.Values[key] = value
}

func (c *currentSession) SetString(key string, value string) {
	c.session.Values[key] = value
}

func (c *currentSession) Get(key string) (any, bool) {
	value, found := c.session.Values[key]
	return value, found
}

func (c *currentSession) GetString(key string) (string, bool) {
	if value, found := c.session.Values[key]; found {
		if txt, ok := value.(string); ok {
			return txt, true
		}
	}
	return "", false
}
