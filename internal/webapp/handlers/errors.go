package handlers

import (
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func exampleError(w http.ResponseWriter, r *http.Request) {
	webapp.RenderError(w, r, nil, "Example error", http.StatusInternalServerError)
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}
