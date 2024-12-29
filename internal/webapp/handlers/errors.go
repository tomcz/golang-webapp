package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func examplePing(w http.ResponseWriter, r *http.Request) {
	webapp.RDebug(r) // don't log these requests
	fmt.Fprintln(w, "Pong")
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	webapp.HttpError(w, r, http.StatusInternalServerError, "Example error", errors.New("oops"))
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("yikes")
}
