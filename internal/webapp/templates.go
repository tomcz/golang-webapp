package webapp

import (
	"fmt"
	"maps"
	"net/http"

	"github.com/tomcz/gotools/buffers"
	"github.com/tomcz/gotools/html"

	"github.com/tomcz/golang-webapp/templates"
)

//goland:noinspection GoBoolExpressions
var tmpl = html.New(templates.FS, templates.Embedded)
var pool = buffers.New()

type renderCfg struct {
	layoutFile   string
	templateName string
	statusCode   int
	contentType  string
	cacheControl string
}

type RenderOpt func(cfg *renderCfg)

func RenderWithoutLayoutFile() RenderOpt {
	return RenderWithLayoutFile("")
}

func RenderWithLayoutFile(layoutFile string) RenderOpt {
	return func(cfg *renderCfg) {
		cfg.layoutFile = layoutFile
	}
}

func RenderWithTemplateName(templateName string) RenderOpt {
	return func(cfg *renderCfg) {
		cfg.templateName = templateName
	}
}

func RenderWithStatusCode(statusCode int) RenderOpt {
	return func(cfg *renderCfg) {
		cfg.statusCode = statusCode
	}
}

func RenderWithContentType(contentType string) RenderOpt {
	return func(cfg *renderCfg) {
		cfg.contentType = contentType
	}
}

func RenderWithCacheControl(cacheControl string) RenderOpt {
	return func(cfg *renderCfg) {
		cfg.cacheControl = cacheControl
	}
}

func Render(w http.ResponseWriter, r *http.Request, templateFile string, data map[string]any, opts ...RenderOpt) {
	cfg := &renderCfg{
		layoutFile:   "layout.gohtml",
		templateName: "main",
		statusCode:   http.StatusOK,
		contentType:  "text/html; charset=utf-8",
		cacheControl: "no-store",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if data == nil {
		data = map[string]any{}
	}
	maps.Copy(data, getSessionData(r))
	// Old-school cache-busting technique: add commit info so that we can use versioned
	// static paths to prevent browsers from using old assets with new deployments.
	data["Commit"] = r.Context().Value(currentCommitKey)

	if !saveSession(w, r) {
		return // error response rendered
	}

	buf := pool.Borrow()
	defer pool.Return(buf)

	err := tmpl.Render(buf, templateFile, data, html.WithLayoutFile(cfg.layoutFile), html.WithTemplateName(cfg.templateName))
	if err != nil {
		err = fmt.Errorf("html.Render: %w", err)
		HttpError(w, r, http.StatusInternalServerError, "Failed to render template", err)
		return
	}

	header := w.Header()
	header.Set("Content-Type", cfg.contentType)
	header.Set("Cache-Control", cfg.cacheControl)
	w.WriteHeader(cfg.statusCode)
	_, _ = buf.WriteTo(w)
}
