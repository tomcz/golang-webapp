package webapp

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

func HttpError(w http.ResponseWriter, r *http.Request, statusCode int, msg string, err error) {
	RSet(r, "error", err)
	msg = fmt.Sprintf("ID: %s\nError: %s\n", ReqID(r), msg)
	http.Error(w, msg, statusCode)
}

func withPanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				stack := string(debug.Stack())
				RSet(r, "panic_stack", stack)
				err := fmt.Errorf("panic: %v", p)
				HttpError(w, r, http.StatusInternalServerError, "Request failed", err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
