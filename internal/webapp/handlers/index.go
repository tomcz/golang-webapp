package handlers

import (
	"net/http"
	"strings"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func showIndex(w http.ResponseWriter, r *http.Request, s webapp.Session) {
	if s.GetString("examples_shown") != "yes" {
		s.AddFlashError("example flash error message")
		s.AddFlashWarning("example flash warning message")
		s.AddFlashMessage("example flash generic message")
		s.AddFlashSuccess("example flash success message")
		s.Set("examples_shown", "yes")
	}
	data := authData(s)
	var opts []webapp.RenderOpt
	if name := s.GetString("Name"); name != "" {
		s.Delete("Name")
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

func updateIndex(w http.ResponseWriter, r *http.Request, s webapp.Session) {
	nameParam := strings.TrimSpace(r.PostFormValue("name"))
	if nameParam != "" {
		s.Set("Name", nameParam)
	}
	redirectToIndex(w, r)
}
