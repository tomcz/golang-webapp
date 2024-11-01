package webapp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
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

type SessionCodec interface {
	Encode(ctx context.Context, session map[string]any, maxAge time.Duration) (string, error)
	Decode(ctx context.Context, value string) (map[string]any, error)
	io.Closer
}

type sessionWrapper struct {
	csrf   CsrfProtection
	codec  SessionCodec
	name   string
	path   string
	maxAge time.Duration
}

type currentSession struct {
	session map[string]any
	wrapper *sessionWrapper
}

func NewSessionWrapper(sessionName string, codec SessionCodec, csrf CsrfProtection) SessionWrapper {
	return &sessionWrapper{
		csrf:   csrf,
		codec:  codec,
		name:   sessionName,
		path:   "/",
		maxAge: 30 * 24 * time.Hour,
	}
}

func (s *sessionWrapper) Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := s.loadSession(r)
		if err != nil {
			RLog(r).Debug("load session failed", "error", err)
			data = make(map[string]any)
		}
		session := &currentSession{session: data, wrapper: s}
		ctx := context.WithValue(r.Context(), currentSessionKey, session)
		r = r.WithContext(ctx)
		if s.csrfSafe(w, r) {
			fn(w, r)
		}
	}
}

func (s *sessionWrapper) loadSession(r *http.Request) (map[string]any, error) {
	cookie, err := r.Cookie(s.name)
	if err != nil {
		return nil, err
	}
	return s.codec.Decode(r.Context(), cookie.Value)
}

func (s *sessionWrapper) saveSession(w http.ResponseWriter, r *http.Request, data map[string]any) error {
	if len(data) == 0 {
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
	value, err := s.codec.Encode(r.Context(), data, s.maxAge)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:     s.name,
		Value:    value,
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
	if s.wrapper.csrf == CsrfDisabled {
		return data
	}
	var csrfToken string
	if s.wrapper.csrf == CsrfPerSession {
		csrfToken = s.GetString(sessionCsrfToken)
	}
	if csrfToken == "" {
		csrfToken = fmt.Sprintf("%x", random.GetRandomBytes(32))
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
		err = fmt.Errorf("save session: %w", err)
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
