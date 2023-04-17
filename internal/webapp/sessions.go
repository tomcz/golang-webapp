package webapp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tomcz/gotools/maps"
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
	sessionCsrfToken    = "_Csrf_Token"
	sessionFlashMessage = "_Flash_Message"
	sessionFlashSuccess = "_Flash_Success"
	sessionFlashWarning = "_Flash_Warning"
	sessionFlashError   = "_Flash_Error"
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
	codec *sessionCodec
	csrf  CsrfProtection
}

type currentSession struct {
	session map[string]any
	csrf    CsrfProtection
	codec   *sessionCodec
}

func NewSessionStore(sessionName, encKey string, csrf CsrfProtection) SessionStore {
	return &sessionStore{
		csrf: csrf,
		codec: &sessionCodec{
			name:   sessionName,
			encKey: keyToBytes(encKey),
			maxAge: 30 * 24 * time.Hour,
			path:   "/",
		},
	}
}

func (s *sessionStore) Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := s.codec.getSession(r)
		if err != nil {
			rlog(r).WithError(err).Debug("session codec failed")
			session = make(map[string]any)
		}
		ctx := context.WithValue(r.Context(), currentSessionKey, &currentSession{
			session: session,
			codec:   s.codec,
			csrf:    s.csrf,
		})
		r = r.WithContext(ctx)
		if s.csrfSafe(w, r) {
			fn(w, r)
		}
	}
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
	cs := CurrentSession(r)
	sessionToken := cs.GetString(sessionCsrfToken)
	if sessionToken == "" {
		err := fmt.Errorf("no csrf token in session")
		RenderErr(w, r, err, "CSRF validation failed", http.StatusBadRequest)
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
		RenderErr(w, r, err, "CSRF validation failed", http.StatusBadRequest)
		return false
	}
	return true
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	if saveSession(w, r) {
		http.Redirect(w, r, url, http.StatusFound)
	}
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
	err := s.codec.setSession(w, r, s.session)
	if err != nil {
		err = fmt.Errorf("session save: %w", err)
		RenderErr(w, r, err, "Failed to save session", http.StatusInternalServerError)
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
	if value, found := c.session[key]; found {
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
	c.session[key] = append(c.getFlash(key), msg)
}

func (c *currentSession) getFlash(key string) []string {
	var res []string
	if flash, ok := c.session[key]; ok {
		res = flash.([]string)
	}
	return res
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
	for _, key := range maps.Keys(c.session) {
		delete(c.session, key)
	}
}
