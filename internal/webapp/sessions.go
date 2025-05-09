package webapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	FlashMessageData = "FlashMessage"
	FlashSuccessData = "FlashSuccess"
	FlashWarningData = "FlashWarning"
	FlashErrorData   = "FlashError"
)

const (
	currentSessionKey   = contextKey("current.session")
	sessionFlashMessage = "_Flash_Message_"
	sessionFlashSuccess = "_Flash_Success_"
	sessionFlashWarning = "_Flash_Warning_"
	sessionFlashError   = "_Flash_Error_"
)

// these cannot be modified via the Session interface
var internalSessionKeys = map[string]bool{
	sessionFlashMessage: true,
	sessionFlashSuccess: true,
	sessionFlashWarning: true,
	sessionFlashError:   true,
}

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
	store  SessionStore
	name   string
	path   string
	maxAge time.Duration
}

type currentSession struct {
	session map[string]any
	wrapper *sessionWrapper
}

func NewSessionWrapper(sessionName string, store SessionStore) SessionWrapper {
	return &sessionWrapper{
		store:  store,
		name:   sessionName,
		path:   "/",
		maxAge: time.Hour,
	}
}

func (s *sessionWrapper) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := &currentSession{session: s.loadSessionData(r), wrapper: s}
		r = r.WithContext(context.WithValue(r.Context(), currentSessionKey, session))
		if s.isCsrfSafe(w, r) {
			next(w, r)
		}
	}
}

func (s *sessionWrapper) loadSessionData(r *http.Request) map[string]any {
	data, err := s.store.Read(s.cookieValue(r))
	if err != nil {
		RLog(r).Debug("load session failed", "error", err)
		data = map[string]any{}
	}
	return data
}

func (s *sessionWrapper) cookieValue(r *http.Request) string {
	cookie, err := r.Cookie(s.name)
	if err != nil {
		return "" // no cookie
	}
	return cookie.Value
}

func (s *sessionWrapper) saveSession(w http.ResponseWriter, r *http.Request, session map[string]any) error {
	oldValue := s.cookieValue(r)
	if len(session) == 0 {
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
	newValue, err := s.store.Write(oldValue, session, s.maxAge)
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

func (s *sessionWrapper) isCsrfSafe(w http.ResponseWriter, r *http.Request) bool {
	if err := csrfCheck(r); err != nil {
		HttpError(w, r, http.StatusBadRequest, "CSRF validation failed", err)
		return false
	}
	return true
}

var csrfSafeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

var csrfSafeFetches = map[string]bool{
	"same-origin": true,
	"none":        true,
}

// adapted from https://github.com/golang/go/issues/73626
func csrfCheck(r *http.Request) error {
	if csrfSafeMethods[r.Method] {
		return nil
	}
	secFetchSite := r.Header.Get("Sec-Fetch-Site")
	if csrfSafeFetches[secFetchSite] {
		return nil
	}
	origin := r.Header.Get("Origin")
	if secFetchSite == "" {
		if origin == "" {
			return errors.New("not a browser request")
		}
		parsed, err := url.Parse(origin)
		if err != nil {
			return fmt.Errorf("bad origin: %w", err)
		}
		if parsed.Host == r.Host {
			return nil
		}
	}
	return fmt.Errorf("Sec-Fetch-Site %q, Origin %q, Host %q", secFetchSite, origin, r.Host)
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
	data[FlashMessageData] = s.popFlash(sessionFlashMessage)
	data[FlashSuccessData] = s.popFlash(sessionFlashSuccess)
	data[FlashWarningData] = s.popFlash(sessionFlashWarning)
	data[FlashErrorData] = s.popFlash(sessionFlashError)
	return data
}

func saveSession(w http.ResponseWriter, r *http.Request) bool {
	s := getSession(r)
	if s == nil {
		return true // no session to save
	}
	err := s.wrapper.saveSession(w, r, s.session)
	if err != nil {
		HttpError(w, r, http.StatusInternalServerError, "Failed to save session", err)
		return false
	}
	return true
}

func (c *currentSession) Set(key string, value any) {
	if internalSessionKeys[key] {
		panic(fmt.Sprintf("cannot override internal session key %q", key))
	}
	c.session[key] = value
}

func (c *currentSession) Delete(key string) {
	if internalSessionKeys[key] {
		panic(fmt.Sprintf("cannot delete internal session key %q", key))
	}
	delete(c.session, key)
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

func (c *currentSession) getFlash(key string) []string {
	if value, found := c.Get(key); found {
		if flash, ok := value.([]string); ok {
			return flash
		}
	}
	return nil
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

func (c *currentSession) popFlash(key string) []string {
	flash := c.getFlash(key)
	delete(c.session, key)
	return flash
}

func (c *currentSession) Clear() {
	c.session = make(map[string]any)
}
