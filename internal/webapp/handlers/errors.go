package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func examplePing(w http.ResponseWriter, r *http.Request) {
	webapp.RDebug(r)
	fmt.Fprintln(w, "Pong")
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	webapp.RenderError(w, r, errors.New("oops"), "Example error", http.StatusInternalServerError)
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}
