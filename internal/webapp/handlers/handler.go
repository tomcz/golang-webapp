package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

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
	r.HandleFunc("/test/{name}", s.Wrap(testName)).Methods("GET").Name("testName")

	return r
}

func showIndex(w http.ResponseWriter, r *http.Request) {
	s := webapp.CurrentSession(r)
	name, _ := s.GetString("name")
	data := map[string]any{"Name": name}
	webapp.Render(w, r, data, "layout.gohtml", "index.gohtml")
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	if name != "" {
		s := webapp.CurrentSession(r)
		s.SetString("name", name)
	}
	webapp.RedirectTo(w, r, "showIndex")
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	webapp.Render500(w, r, "Example error")
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}

func testName(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	s := webapp.CurrentSession(r)
	s.SetString("name", name)
	webapp.RedirectTo(w, r, "showIndex")
}
