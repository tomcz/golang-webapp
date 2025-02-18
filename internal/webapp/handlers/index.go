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
	data := authData(r)
	var opts []webapp.RenderOpt
	if name := session.GetString("Name"); name != "" {
		session.Delete("Name")
		data["Name"] = name
		if isPartial(r) {
			opts = append(opts,
				webapp.RenderWithTemplateName("hello"),
				webapp.RenderWithoutLayoutFile(),
			)
		}
	}
	webapp.Render(w, r, "index.gohtml", data, opts...)
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	nameParam := strings.TrimSpace(r.PostFormValue("name"))
	if nameParam == "" {
		webapp.CurrentSession(r).Set("Name", nameParam)
	}
	redirectToIndex(w, r)
}
