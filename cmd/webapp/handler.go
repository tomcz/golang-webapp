package main

import (
	"fmt"
	"net/http"
)

func newHandler() http.Handler {
	r := http.NewServeMux()
	r.HandleFunc("/index", index)
	r.Handle("/", http.RedirectHandler("/index", http.StatusFound))
	return r
}

func index(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintln(w, "hello")
}
