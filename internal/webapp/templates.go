package webapp

import (
	"fmt"
	"html/template"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"

	"github.com/tomcz/golang-webapp/templates"
)

type renderCfg struct {
	layoutFile   string
	templateName string
	statusCode   int
	contentType  string
	cacheControl string
	unbuffered   bool
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

func RenderWithoutBuffer() RenderOpt {
	return func(cfg *renderCfg) {
		cfg.unbuffered = true
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

	tmpl, err := newTemplate(cfg.layoutFile, templateFile)
	if err != nil {
		err = fmt.Errorf("template.new: %w", err)
		HttpError(w, r, http.StatusInternalServerError, "Failed to create template", err)
		return
	}

	// We buffer template execution output by default to avoid writing incomplete or malformed
	// content to the response, but sometimes we need to render a huge data set without buffering.
	if cfg.unbuffered {
		writeUnbuffered(w, r, tmpl, data, cfg)
		return
	}
	writeBuffered(w, r, tmpl, data, cfg)
}

// Generally we will write once and read many times so using a sync.Map
// is preferred as it reduces lock contention compared to a sync.RWMutex.
var tmplCache sync.Map

func newTemplate(templatePaths ...string) (*template.Template, error) {
	// There is no need to recreate templates in embedded builds
	// since they're not going to change between renders.
	var cacheKey string
	// No goland, it isn't always false.
	//goland:noinspection GoBoolExpressions
	if templates.Embedded {
		cacheKey = strings.Join(templatePaths, ",")
		if cached, ok := tmplCache.Load(cacheKey); ok {
			return cached.(*template.Template), nil
		}
	}
	tmpl := template.New("")
	for _, path := range templatePaths {
		if path == "" {
			continue
		}
		buf, err := readTemplate(path)
		if err != nil {
			return nil, err
		}
		tmpl, err = tmpl.Parse(string(buf))
		if err != nil {
			return nil, fmt.Errorf("%s parse failed: %w", path, err)
		}
	}
	// No goland, it isn't always false.
	//goland:noinspection GoBoolExpressions
	if templates.Embedded {
		tmplCache.Store(cacheKey, tmpl)
	}
	return tmpl, nil
}

func readTemplate(path string) ([]byte, error) {
	in, err := templates.FS.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s open failed: %w", path, err)
	}
	defer in.Close() //nolint

	buf, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("%s read failed: %w", path, err)
	}
	return buf, nil
}

func writeUnbuffered(w http.ResponseWriter, r *http.Request, tmpl *template.Template, data map[string]any, cfg *renderCfg) {
	writeHeaders(w, cfg)
	err := tmpl.ExecuteTemplate(w, cfg.templateName, data)
	if err != nil {
		RLog(r).Error("unbuffered write failed", "error", err)
	}
}

func writeBuffered(w http.ResponseWriter, r *http.Request, tmpl *template.Template, data map[string]any, cfg *renderCfg) {
	buf := BufBorrow()
	defer BufReturn(buf)

	err := tmpl.ExecuteTemplate(buf, cfg.templateName, data)
	if err != nil {
		err = fmt.Errorf("template.exec: %w", err)
		HttpError(w, r, http.StatusInternalServerError, "Failed to execute template", err)
		return
	}

	writeHeaders(w, cfg)
	_, err = buf.WriteTo(w)
	if err != nil {
		RLog(r).Error("buffered write failed", "error", err)
	}
}

func writeHeaders(w http.ResponseWriter, cfg *renderCfg) {
	header := w.Header()
	header.Set("Content-Type", cfg.contentType)
	header.Set("Cache-Control", cfg.cacheControl)
	w.WriteHeader(cfg.statusCode)
}
