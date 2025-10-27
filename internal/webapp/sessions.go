package webapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

const currentSessionKey = contextKey("current.session")

const (
	FlashMessageData = "FlashMessage"
	FlashSuccessData = "FlashSuccess"
	FlashWarningData = "FlashWarning"
	FlashErrorData   = "FlashError"
)

const (
	sessionFlashMessage = "_Flash_Message_"
	sessionFlashSuccess = "_Flash_Success_"
	sessionFlashWarning = "_Flash_Warning_"
	sessionFlashError   = "_Flash_Error_"
)

type HandlerWithSession func(w http.ResponseWriter, r *http.Request, s Session)

type SessionProvider func(next HandlerWithSession) http.HandlerFunc

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

// RegisterWithSessionSerializer a prototype of something you expect to store in the session.
func RegisterWithSessionSerializer(prototype any) {
	gob.Register(prototype)
}

type SessionCookieConfig struct {
	CookieName string
	AuthKey    string
	EncKey     string
	MaxAge     time.Duration
	Secure     bool
}

func UseSessionCookies(cfg SessionCookieConfig) SessionProvider {
	store := newCookieStore(cfg)
	return func(next HandlerWithSession) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			session, _ := store.New(r, cfg.CookieName)
			ctx := context.WithValue(r.Context(), currentSessionKey, session)
			next(w, r.WithContext(ctx), &sessionWrapper{session})
		}
	}
}

func newCookieStore(cfg SessionCookieConfig) *sessions.CookieStore {
	store := sessions.NewCookieStore(decodeSessionKey(cfg.AuthKey), decodeSessionKey(cfg.EncKey))
	maxAge := int(cfg.MaxAge.Seconds())
	store.Options.MaxAge = maxAge
	store.Options.HttpOnly = true
	store.Options.Path = "/"
	if cfg.Secure {
		store.Options.Secure = true
		store.Options.SameSite = http.SameSiteNoneMode
	} else {
		store.Options.Secure = false
		store.Options.SameSite = http.SameSiteDefaultMode
	}
	store.MaxAge(maxAge)
	return store
}

func NewSessionKey() string {
	return base64.URLEncoding.EncodeToString(newSessionKey())
}

func decodeSessionKey(key string) []byte {
	if key == "" {
		return newSessionKey()
	}
	buf, err := base64.URLEncoding.DecodeString(key)
	if err == nil && len(buf) == 32 {
		return buf
	}
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

func newSessionKey() []byte {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return buf
}

func getSession(r *http.Request) *sessions.Session {
	if session, ok := r.Context().Value(currentSessionKey).(*sessions.Session); ok {
		return session
	}
	return nil
}

func getSessionData(r *http.Request) map[string]any {
	session := getSession(r)
	if session == nil {
		return nil
	}
	return map[string]any{
		FlashMessageData: session.Flashes(sessionFlashMessage),
		FlashSuccessData: session.Flashes(sessionFlashSuccess),
		FlashWarningData: session.Flashes(sessionFlashWarning),
		FlashErrorData:   session.Flashes(sessionFlashError),
	}
}

func saveSession(w http.ResponseWriter, r *http.Request) bool {
	session := getSession(r)
	if session == nil {
		return true
	}
	if err := session.Save(r, w); err != nil {
		HttpError(w, r, http.StatusInternalServerError, "failed to save session", err)
		return false
	}
	return true
}

type sessionWrapper struct {
	delegate *sessions.Session
}

func (s *sessionWrapper) Set(key string, value any) {
	s.delegate.Values[key] = value
}

func (s *sessionWrapper) Get(key string) (any, bool) {
	value, ok := s.delegate.Values[key]
	return value, ok
}

func (s *sessionWrapper) GetString(key string) string {
	if value, ok := s.delegate.Values[key].(string); ok {
		return value
	}
	return ""
}

func (s *sessionWrapper) AddFlashMessage(msg string) {
	s.delegate.AddFlash(msg, sessionFlashMessage)
}

func (s *sessionWrapper) AddFlashSuccess(msg string) {
	s.delegate.AddFlash(msg, sessionFlashSuccess)
}

func (s *sessionWrapper) AddFlashWarning(msg string) {
	s.delegate.AddFlash(msg, sessionFlashWarning)
}

func (s *sessionWrapper) AddFlashError(msg string) {
	s.delegate.AddFlash(msg, sessionFlashError)
}

func (s *sessionWrapper) Delete(key string) {
	delete(s.delegate.Values, key)
}

func (s *sessionWrapper) Clear() {
	s.delegate.Values = make(map[any]any)
}
