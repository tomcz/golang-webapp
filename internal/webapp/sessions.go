package webapp

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"time"
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
	Wrap(next http.HandlerFunc) http.HandlerFunc
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
	Write(value string, session map[string]any, maxAge time.Duration) (string, error)
	Read(value string) (map[string]any, error)
	Delete(value string)
	io.Closer
}

type sessionWrapper struct {
	csrf   CsrfProtection
	store  SessionStore
	name   string
	path   string
	maxAge time.Duration
}

type currentSession struct {
	session map[string]any
	wrapper *sessionWrapper
}

func NewSessionWrapper(sessionName string, store SessionStore, csrf CsrfProtection) SessionWrapper {
	return &sessionWrapper{
		csrf:   csrf,
		store:  store,
		name:   sessionName,
		path:   "/",
		maxAge: time.Hour,
	}
}

func (s *sessionWrapper) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := s.loadSession(r)
		if err != nil {
			RLog(r).Debug("load session failed", "error", err)
			data = make(map[string]any)
		}
		session := &currentSession{session: data, wrapper: s}
		r = r.WithContext(context.WithValue(r.Context(), currentSessionKey, session))
		if s.isCsrfSafe(w, r, session) {
			next(w, r)
		}
	}
}

func (s *sessionWrapper) cookieValue(r *http.Request) string {
	cookie, err := r.Cookie(s.name)
	if err != nil {
		return "" // no cookie
	}
	return cookie.Value
}

func (s *sessionWrapper) loadSession(r *http.Request) (map[string]any, error) {
	return s.store.Read(s.cookieValue(r))
}

func (s *sessionWrapper) saveSession(w http.ResponseWriter, r *http.Request, data map[string]any) error {
	oldValue := s.cookieValue(r)
	if len(data) == 0 {
		s.store.Delete(oldValue)
		cookie := &http.Cookie{
			Name:     s.name,
			Path:     s.path,
			MaxAge:   -1,
			Secure:   r.URL.Scheme == "https",
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
		return nil
	}
	newValue, err := s.store.Write(oldValue, data, s.maxAge)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:     s.name,
		Value:    newValue,
		Path:     s.path,
		Expires:  time.Now().Add(s.maxAge),
		MaxAge:   int(s.maxAge.Seconds()),
		Secure:   r.URL.Scheme == "https",
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	return nil
}

// according to RFC-7231
var csrfSafeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

func (s *sessionWrapper) isCsrfSafe(w http.ResponseWriter, r *http.Request, cs *currentSession) bool {
	if csrfSafeMethods[r.Method] {
		return true
	}
	if s.csrf == CsrfDisabled {
		return true
	}
	requestToken := r.Header.Get(CsrfHttpHeader)
	if requestToken == "" {
		requestToken = r.FormValue(CsrfFormToken)
	}
	if requestToken == "" {
		return csrfFailed(w, r, errors.New("no csrf token in request"))
	}
	sessionToken := cs.GetString(sessionCsrfToken)
	if sessionToken == "" {
		return csrfFailed(w, r, errors.New("no csrf token in session"))
	}
	if s.csrf == CsrfPerRequest {
		cs.Delete(sessionCsrfToken)
	}
	if subtle.ConstantTimeCompare([]byte(requestToken), []byte(sessionToken)) != 0 {
		return true
	}
	err := errors.New("csrf token mismatch")
	if s.csrf == CsrfPerRequest {
		if fail := s.saveSession(w, r, cs.session); fail != nil {
			err = errors.Join(err, fail)
		}
	}
	return csrfFailed(w, r, err)
}

func csrfFailed(w http.ResponseWriter, r *http.Request, err error) bool {
	RenderError(w, r, err, "CSRF validation failed", http.StatusBadRequest)
	return false
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
	if s.wrapper.csrf == CsrfDisabled {
		return data
	}
	var csrfToken string
	if s.wrapper.csrf == CsrfPerSession {
		csrfToken = s.GetString(sessionCsrfToken)
	}
	if csrfToken == "" {
		csrfToken = longRandomText()
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
	err := s.wrapper.saveSession(w, r, s.session)
	if err != nil {
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
