package internal

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

func panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				stack := string(debug.Stack())
				rset(r, "panic_stack", stack)
				rerr(r, fmt.Errorf("panic: %v", p))
				render500(w, r, "Request failed")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func rerr(r *http.Request, err error) {
	rset(r, "err", err)
}

func render500(w http.ResponseWriter, r *http.Request, msg string) {
	if id := rid(r); id != "" {
		msg = fmt.Sprintf("ID: %s\nError: %s\n", id, msg)
	}
	http.Error(w, msg, http.StatusInternalServerError)
}

func renderErr(w http.ResponseWriter, r *http.Request, err error, msg string) {
	rerr(r, err)
	render500(w, r, msg)
}
