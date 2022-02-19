package main

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/tomcz/golang-webapp/static"
)

func newHandler(s *sessionStore, isDev bool) http.Handler {
	staticAssets := http.StripPrefix("/static/", http.FileServer(static.FS))
	staticAssets = staticCacheControl(staticAssets, isDev)
	r := mux.NewRouter()
	r.PathPrefix("/static").Handler(staticAssets)
	r.HandleFunc("/index", s.wrap(showIndex)).Methods("GET")
	r.HandleFunc("/index", s.wrap(updateIndex)).Methods("POST")
	r.HandleFunc("/error", exampleError).Methods("GET")
	r.HandleFunc("/panic", examplePanic).Methods("GET")
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound))
	r.Use(noStoreCacheControl)
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
	renderError(w, r, errors.New("wibble"), "example error")
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}
