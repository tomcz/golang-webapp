package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func newHandler(s *sessionStore) http.Handler {
	r := mux.NewRouter()
	registerStaticAssetRoutes(r)

	// no session
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound)).Name("root")
	r.HandleFunc("/error", exampleError).Methods("GET").Name("exampleError")
	r.HandleFunc("/panic", examplePanic).Methods("GET").Name("examplePanic")

	// with session
	rs := r.NewRoute().Subrouter()
	rs.HandleFunc("/index", showIndex).Methods("GET").Name("showIndex")
	rs.HandleFunc("/index", updateIndex).Methods("POST").Name("updateIndex")
	rs.HandleFunc("/test/{name}", testName).Methods("GET").Name("testName")
	rs.Use(s.wrapHandler)

	r.Use(setRouter(r), noStoreCacheControl, setCurrentRouteName)
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
	redirectToRoute(w, r, "showIndex")
}

func exampleError(w http.ResponseWriter, r *http.Request) {
	render500(w, r, "Example error")
}

func examplePanic(http.ResponseWriter, *http.Request) {
	panic("wobble")
}

func testName(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	s := currentSession(r)
	s.Values["name"] = name
	redirectToRoute(w, r, "showIndex")
}
