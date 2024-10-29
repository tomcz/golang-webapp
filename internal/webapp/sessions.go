package webapp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

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

type SessionWrapper interface {
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

type SessionStore interface {
	SetSession(w http.ResponseWriter, r *http.Request, session map[string]any) error
	GetSession(r *http.Request) (map[string]any, error)
}

type sessionWrapper struct {
	csrf  CsrfProtection
	store SessionStore
}

type currentSession struct {
	session map[string]any
	csrf    CsrfProtection
	store   SessionStore
}

func NewSessionWrapper(codec SessionStore, csrf CsrfProtection) SessionWrapper {
	return &sessionWrapper{
		csrf:  csrf,
		store: codec,
	}
}

func (s *sessionWrapper) Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := s.store.GetSession(r)
		if err != nil {
			RLog(r).Debug("session codec failed", "error", err)
			session = make(map[string]any)
		}
		ctx := context.WithValue(r.Context(), currentSessionKey, &currentSession{
			session: session,
			csrf:    s.csrf,
			store:   s.store,
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

func (s *sessionWrapper) csrfSafe(w http.ResponseWriter, r *http.Request) bool {
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
	err := s.store.SetSession(w, r, s.session)
	if err != nil {
		err = fmt.Errorf("session save: %w", err)
		RenderError(w, r, err, "Failed to save session", http.StatusInternalServerError)
		return false
	}
	return true
}

func (c *currentSession) Set(key string, value any) {
	c.session[key] = value
}

func (c *currentSession) Get(key string) (any, bool) {
	value, found := c.session[key]
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
	c.Set(key, append(c.getFlash(key), msg))
}

func (c *currentSession) getFlash(key string) []string {
	if value, found := c.Get(key); found {
		if flash, ok := value.([]string); ok {
			return flash
		}
	}
	return nil
}

func (c *currentSession) popFlash(key string) []string {
	flash := c.getFlash(key)
	c.Delete(key)
	return flash
}

func (c *currentSession) Delete(key string) {
	delete(c.session, key)
}

func (c *currentSession) Clear() {
	c.session = make(map[string]any)
}
