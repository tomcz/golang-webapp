package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func examplePing(w http.ResponseWriter, r *http.Request) {
	webapp.RDebug(r)        //nolog
	fmt.Fprintln(w, "Pong") //nolint
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	webapp.HttpError(w, r, http.StatusInternalServerError, "Example error", errors.New("oops"))
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("yikes")
}
