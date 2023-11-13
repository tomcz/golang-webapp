package handlers

import (
	"net/http"
	"strings"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func showIndex(w http.ResponseWriter, r *http.Request) {
	username := webapp.CurrentSession(r).GetString(authUserKey)
	data := map[string]any{authUserKey: username}
	webapp.Render(w, r, "index.gohtml", data)
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		name = "World"
	}
	data := map[string]any{"Name": name}
	webapp.Render(w, r, "hello.gohtml", data,
		webapp.RenderWithTemplateName("body"),
		webapp.RenderWithLayoutFile(""),
	)
}
