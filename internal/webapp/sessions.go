package webapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const keyLength = 32 // bytes -> AES-256

const (
	CsrfFormToken  = "_csrf_token"
	CsrfHttpHeader = "X-Csrf-Token"
	CsrfTokenKey   = "CsrfToken"
)

type CsrfProtection int

const (
	CsrfDisabled CsrfProtection = iota
	CsrfPerRequest
	CsrfPerSession
)

const (
	FlashMessageKey = "FlashMessage"
	FlashSuccessKey = "FlashSuccess"
	FlashWarningKey = "FlashWarning"
	FlashErrorKey   = "FlashError"
)

const (
	currentSessionKey   = contextKey("current.session")
	sessionCsrfToken    = "_Csrf_Token_"
	sessionFlashMessage = "_Flash_Message_"
	sessionFlashSuccess = "_Flash_Success_"
	sessionFlashWarning = "_Flash_Warning_"
	sessionFlashError   = "_Flash_Error_"
)

type SessionStore interface {
	Wrap(fn http.HandlerFunc) http.HandlerFunc
}

type Session interface {
	Set(key string, value any)
	Get(key string) (any, bool)
	GetString(key string) string
	AddFlashMessage(msg string)
	AddFlashSuccess(msg string)
	AddFlashWarning(msg string)
	AddFlashError(msg string)
	Delete(key string)
	Clear()
}

type sessionStore struct {
	csrf  CsrfProtection
	store sessions.Store
	name  string
}

type currentSession struct {
	csrf    CsrfProtection
	session *sessions.Session
}

func EncodeRandomKey() string {
	return base64.StdEncoding.EncodeToString(randomKey())
}

func randomKey() []byte {
	key := securecookie.GenerateRandomKey(keyLength)
	if len(key) == 0 {
		panic("cannot generate a random key")
	}
	return key
}

func keyBytes(key string) []byte {
	if key == "" {
		return randomKey()
	}
	buf, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(buf) != keyLength {
		return randomKey()
	}
	return buf
}

func NewSessionStore(cookieName, authKey, encKey string, csrf CsrfProtection) (SessionStore, error) {
	store := sessions.NewCookieStore(keyBytes(authKey), keyBytes(encKey))
	store.MaxAge(int((30 * 24 * time.Hour).Seconds())) // 30 days
	return &sessionStore{
		csrf:  csrf,
		store: store,
		name:  cookieName,
	}, nil
}

func (s *sessionStore) Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

// according to RFC-7231
var csrfSafeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

func (s *sessionStore) csrfSafe(w http.ResponseWriter, r *http.Request) bool {
	if csrfSafeMethods[r.Method] {
		return true
	}
	if s.csrf == CsrfDisabled {
		return true
	}
	cs := CurrentSession(r)
	sessionToken := cs.GetString(sessionCsrfToken)
	if sessionToken == "" {
		err := fmt.Errorf("no csrf token in session")
		RenderError(w, r, err, "CSRF validation failed", http.StatusBadRequest)
		return false
	}
	if s.csrf == CsrfPerRequest {
		cs.Delete(sessionCsrfToken)
	}
	requestToken := r.Header.Get(CsrfHttpHeader)
	if requestToken == "" {
		requestToken = r.FormValue(CsrfFormToken)
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

func CurrentSession(r *http.Request) Session {
	s := getSession(r)
	if s == nil {
		panic("no current session; is this handler wrapped?")
	}
	return s
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
	data[FlashMessageKey] = s.popFlash(sessionFlashMessage)
	data[FlashSuccessKey] = s.popFlash(sessionFlashSuccess)
	data[FlashWarningKey] = s.popFlash(sessionFlashWarning)
	data[FlashErrorKey] = s.popFlash(sessionFlashError)
	if s.csrf == CsrfDisabled {
		return data
	}
	var csrfToken string
	if s.csrf == CsrfPerSession {
		csrfToken = s.GetString(sessionCsrfToken)
	}
	if csrfToken == "" {
		csrfToken = uuid.NewString()
		s.Set(sessionCsrfToken, csrfToken)
	}
	data[CsrfTokenKey] = csrfToken
	return data
}

func saveSession(w http.ResponseWriter, r *http.Request) bool {
	s := getSession(r)
	if s == nil {
		return true // no session to save
	}
	err := s.session.Save(r, w)
	if err != nil {
		err = fmt.Errorf("session save: %w", err)
		RenderError(w, r, err, "Failed to save session", http.StatusInternalServerError)
		return false
	}
	return true
}

func (c *currentSession) Set(key string, value any) {
	c.session.Values[key] = value
}

func (c *currentSession) Get(key string) (any, bool) {
	value, found := c.session.Values[key]
	return value, found
}

func (c *currentSession) GetString(key string) string {
	if value, found := c.Get(key); found {
		if txt, ok := value.(string); ok {
			return txt
		}
	}
	return ""
}

func (c *currentSession) AddFlashMessage(msg string) {
	c.addFlash(sessionFlashMessage, msg)
}

func (c *currentSession) AddFlashSuccess(msg string) {
	c.addFlash(sessionFlashSuccess, msg)
}

func (c *currentSession) AddFlashWarning(msg string) {
	c.addFlash(sessionFlashWarning, msg)
}

func (c *currentSession) AddFlashError(msg string) {
	c.addFlash(sessionFlashError, msg)
}

func (c *currentSession) addFlash(key, msg string) {
	c.session.AddFlash(msg, key)
}

func (c *currentSession) popFlash(key string) []any {
	return c.session.Flashes(key)
}

func (c *currentSession) Delete(key string) {
	delete(c.session.Values, key)
}

func (c *currentSession) Clear() {
	c.session.Options.MaxAge = -1
}
