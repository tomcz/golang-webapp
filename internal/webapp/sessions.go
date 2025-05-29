package webapp

import (
	"context"
	"net/http"

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

func CurrentSession(r *http.Request) Session {
	session := getSession(r)
	if session == nil {
		panic("no current session")
	}
	return &sessionWrapper{session}
}

func sessionMiddleware(store sessions.Store, sessionName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.New(r, sessionName)
		ctx := context.WithValue(r.Context(), currentSessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
