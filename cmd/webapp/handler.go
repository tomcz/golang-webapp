package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func newHandler(s *sessionStore) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/index", s.wrap(showIndex)).Methods("GET").Name("showIndex")
	r.HandleFunc("/index", s.wrap(updateIndex)).Methods("POST").Name("updateIndex")
	r.HandleFunc("/error", exampleError).Methods("GET").Name("exampleError")
	r.HandleFunc("/panic", examplePanic).Methods("GET").Name("examplePanic")
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound))
	r.Use(noStoreCacheControl, setCurrentRouteName)
	registerStaticAssetRoutes(r)
	return r
}

func showIndex(w http.ResponseWriter, r *http.Request) {
	name := ""
	s := currentSession(r)
	if v, ok := s.Values["name"].(string); ok {
		delete(s.Values, "name")
		name = v
	}
	data := renderData{"Name": name}
	render(w, r, data, "layout.gohtml", "index.gohtml")
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	if name != "" {
		s := currentSession(r)
		s.Values["name"] = name
	}
	redirect(w, r, "/index")
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	render500(w, r, "Example error")
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}
