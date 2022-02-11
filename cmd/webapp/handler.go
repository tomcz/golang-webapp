package main

import (
	"fmt"
	"net/http"
)

func newHandler(s *sessionsStore) http.Handler {
	r := http.NewServeMux()
	r.HandleFunc("/index", s.wrap(index))
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound))
	return r
}

func index(w http.ResponseWriter, r *http.Request) {
	if saveSession(w, r) {
		fmt.Fprintln(w, "hello")
	}
}
