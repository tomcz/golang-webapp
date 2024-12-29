package handlers

import (
	"net/http"
	"strings"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func showIndex(w http.ResponseWriter, r *http.Request) {
	session := webapp.CurrentSession(r)
	if session.GetString("examples_shown") != "yes" {
		session.AddFlashError("example flash error message")
		session.AddFlashWarning("example flash warning message")
		session.AddFlashMessage("example flash generic message")
		session.AddFlashSuccess("example flash success message")
		session.Set("examples_shown", "yes")
	}
	webapp.Render(w, r, "index.gohtml", authData(r))
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	nameParam := strings.TrimSpace(r.PostFormValue("name"))
	if nameParam == "" {
		nameParam = "World"
	}
	var opts []webapp.RenderOpt
	if isPartial(r) {
		opts = append(opts,
			webapp.RenderWithTemplateName("body"),
			webapp.RenderWithoutLayoutFile(),
		)
	}
	data := authData(r)
	data["Name"] = nameParam
	webapp.Render(w, r, "hello.gohtml", data, opts...)
}
