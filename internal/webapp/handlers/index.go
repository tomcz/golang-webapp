package handlers

import (
	"fmt"
	"net/http"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func showIndex(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{authUserKey: webapp.CurrentSession(r).GetString(authUserKey)}
	webapp.Render(w, r, data, "layout.gohtml", "index.gohtml")
}

func updateIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	if name != "" {
		s := webapp.CurrentSession(r)
		s.AddFlashSuccess(fmt.Sprintf("Hello %s", name))
	}
	webapp.Redirect(w, r, "/index")
}
