package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func newHandler(s *sessionsStore) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/index", s.wrap(showIndex)).Methods("GET")
	r.HandleFunc("/index", s.wrap(updateIndex)).Methods("POST")
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound))
	return r
}

func showIndex(w http.ResponseWriter, r *http.Request) {
	name := "Wibble"
	s := getSession(r)
	if v, ok := s.Values["name"].(string); ok {
		delete(s.Values, "name")
		name = v
	}
	data := map[string]interface{}{
		"Name": name,
	}
	renderTemplate(w, r, data, "layout.gohtml", "index.gohtml")
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	if name != "" {
		s := getSession(r)
		s.Values["name"] = name
	}
	redirect(w, r, "/index")
}
