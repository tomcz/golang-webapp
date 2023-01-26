package webapp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/tomcz/gotools/maps"
)

const (
	currentSessionKey = contextKey("current.session")
	csrfFormToken     = "_csrf_token"
	csrfHttpHeader    = "X-Csrf-Token"
	csrfSessionValue  = "CsrfToken"
)

type CsrfProtection int

const (
	CsrfDisabled CsrfProtection = iota
	CsrfPerRequest
	CsrfPerSession
)

type SessionStore interface {
	Wrap(fn http.HandlerFunc) http.HandlerFunc
}

type Session interface {
	Set(key string, value any)
	Get(key string) (any, bool)
	GetString(key string) string
	Delete(key string)
	AddFlash(msg string)
}

type sessionStore struct {
	store sessions.Store
	name  string
	csrf  CsrfProtection
}

type currentSession struct {
	session *sessions.Session
	csrf    CsrfProtection
}

func NewSessionStore(sessionName, authKey, encKey string, csrf CsrfProtection) SessionStore {
	return &sessionStore{
		store: sessions.NewCookieStore(keyToBytes(authKey), keyToBytes(encKey)),
		name:  sessionName,
		csrf:  csrf,
	}
}

func (s *sessionStore) Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// We're ignoring the error resulted from decoding an existing session
		// since Get() always returns a session, even if empty.
		session, _ := s.store.Get(r, s.name)
		ctx := context.WithValue(r.Context(), currentSessionKey, &currentSession{
			session: session,
			csrf:    s.csrf,
		})
		r = r.WithContext(ctx)
		if s.csrfSafe(w, r) {
			fn(w, r)
		}
	}
}

func CurrentSession(r *http.Request) Session {
	s := getSession(r)
	if s == nil {
		panic("no current session; is this handler wrapped?")
	}
	return s
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

// according to RFC-7231
var csrfSafeMethods = maps.NewSet("GET", "HEAD", "OPTIONS", "TRACE")

func (s *sessionStore) csrfSafe(w http.ResponseWriter, r *http.Request) bool {
	if csrfSafeMethods[r.Method] {
		return true
	}
	if s.csrf == CsrfDisabled {
		return true
	}
	ss := CurrentSession(r)
	sessionToken := ss.GetString(csrfSessionValue)
	if sessionToken == "" {
		err := fmt.Errorf("no csrf token in session")
		RenderError(w, r, err, "CSRF validation failed", http.StatusBadRequest)
		return false
	}
	if s.csrf == CsrfPerRequest {
		ss.Delete(csrfSessionValue)
	}
	requestToken := r.Header.Get(csrfHttpHeader)
	if requestToken == "" {
		requestToken = r.FormValue(csrfFormToken)
	}
	var err error
	if requestToken == "" {
		err = fmt.Errorf("no csrf token in request")
	}
	if requestToken != sessionToken {
		err = fmt.Errorf("csrf token mismatch")
	}
	if err != nil {
		if s.csrf == CsrfPerRequest && !saveSession(w, r) {
			return false
		}
		RenderError(w, r, err, "CSRF validation failed", http.StatusBadRequest)
		return false
	}
	return true
}

func getSession(r *http.Request) *currentSession {
	if s, ok := r.Context().Value(currentSessionKey).(*currentSession); ok {
		return s
	}
	return nil
}

func getSessionData(r *http.Request) map[string]any {
	data := map[string]any{}
	s := getSession(r)
	if s == nil {
		return data
	}
	data["Flash"] = s.session.Flashes()
	if s.csrf == CsrfDisabled {
		return data
	}
	var csrfToken string
	if s.csrf == CsrfPerSession {
		csrfToken = s.GetString(csrfSessionValue)
	}
	if csrfToken == "" {
		csrfToken = uuid.New().String()
		s.Set(csrfSessionValue, csrfToken)
	}
	data[csrfSessionValue] = csrfToken
	return data
}

func saveSession(w http.ResponseWriter, r *http.Request) bool {
	s := getSession(r)
	if s == nil {
		return true // no session to save
	}
	err := s.session.Save(r, w)
	if err == nil {
		return true // saved properly
	}
	RenderError(w, r, err, "failed to save session", http.StatusInternalServerError)
	return false
}

func (c *currentSession) Set(key string, value any) {
	c.session.Values[key] = value
}

func (c *currentSession) Get(key string) (any, bool) {
	value, found := c.session.Values[key]
	return value, found
}

func (c *currentSession) GetString(key string) string {
	if value, found := c.session.Values[key]; found {
		if txt, ok := value.(string); ok {
			return txt
		}
	}
	return ""
}

func (c *currentSession) Delete(key string) {
	delete(c.session.Values, key)
}

func (c *currentSession) AddFlash(msg string) {
	c.session.AddFlash(msg)
}
