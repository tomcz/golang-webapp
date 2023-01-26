package handlers

import (
	"fmt"
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func NewHandler(s webapp.SessionStore) http.Handler {
	r := webapp.NewRouter()

	// no session
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound)).Name("root")
	r.HandleFunc("/error", exampleError).Methods("GET").Name("exampleError")
	r.HandleFunc("/panic", examplePanic).Methods("GET").Name("examplePanic")

	// with session
	r.HandleFunc("/index", s.Wrap(showIndex)).Methods("GET").Name("showIndex")
	r.HandleFunc("/index", s.Wrap(updateIndex)).Methods("POST").Name("updateIndex")

	return r
}

func showIndex(w http.ResponseWriter, r *http.Request) {
	webapp.Render(w, r, nil, "layout.gohtml", "index.gohtml")
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	if name != "" {
		s := webapp.CurrentSession(r)
		s.AddFlash(fmt.Sprintf("Hello %s", name))
	}
	webapp.RedirectTo(w, r, "showIndex")
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	webapp.RenderErr(w, r, nil, "Example error", http.StatusInternalServerError)
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}
