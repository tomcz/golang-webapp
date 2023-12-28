package handlers

import (
	"net/http"
	"strings"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func showIndex(w http.ResponseWriter, r *http.Request) {
	session := webapp.CurrentSession(r)
	session.AddFlashError("example flash error message")
	session.AddFlashWarning("example flash warning message")
	session.AddFlashMessage("example flash generic message")
	session.AddFlashSuccess("example flash success message")
	username := session.GetString(authUserKey)
	data := map[string]any{authUserKey: username}
	webapp.Render(w, r, "index.gohtml", data)
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		name = "World"
	}
	var opts []webapp.RenderOpt
	if r.Header.Get("Hx-Request") == "true" {
		opts = append(opts,
			webapp.RenderTemplateName("body"),
			webapp.RenderLayoutFile(""),
		)
	}
	data := map[string]any{"Name": name}
	webapp.Render(w, r, "hello.gohtml", data, opts...)
}
