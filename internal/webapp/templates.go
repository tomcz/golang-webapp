package webapp

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/templates"
)

func RenderError(w http.ResponseWriter, r *http.Request, err error, msg string, statusCode int) {
	RSet(r, "error", err)
	msg = fmt.Sprintf("ID: %s\nError: %s\n", RId(r), msg)
	http.Error(w, msg, statusCode)
}

type renderCfg struct {
	layoutFile   string
	templateName string
	statusCode   int
	contentType  string
	unbuffered   bool
}

type RenderOpt func(cfg *renderCfg)

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
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if data == nil {
		data = map[string]any{}
	}
	for key, value := range getSessionData(r) {
		data[key] = value
	}
	if !saveSession(w, r) {
		return
	}

	// add commit info so that we can use versioned static paths
	// to prevent browsers using old assets with new deployments
	data["Commit"] = build.Commit()

	tmpl, err := newTemplate(cfg.layoutFile, templateFile)
	if err != nil {
		err = fmt.Errorf("template new: %w", err)
		RenderError(w, r, err, "Failed to create template", http.StatusInternalServerError)
		return
	}

	if cfg.unbuffered {
		w.Header().Set("Content-Type", cfg.contentType)
		w.WriteHeader(cfg.statusCode)
		err = tmpl.ExecuteTemplate(w, cfg.templateName, data)
		if err != nil {
			RLog(r).Error("unbuffered write failed", "error", err)
		}
		return
	}

	// buffer template execution output to avoid writing
	// incomplete or malformed content to the response
	buf := BufBorrow()
	defer BufReturn(buf)
	err = tmpl.ExecuteTemplate(buf, cfg.templateName, data)
	if err != nil {
		err = fmt.Errorf("template exec: %w", err)
		RenderError(w, r, err, "Failed to execute template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", cfg.contentType)
	w.WriteHeader(cfg.statusCode)
	_, err = buf.WriteTo(w)
	if err != nil {
		RLog(r).Error("buffered write failed", "error", err)
	}
}

// Generally we will write once and read many times so using a sync.Map
// is preferred as it reduces lock contention compared to a sync.RWMutex.
var tmplCache sync.Map

func newTemplate(templatePaths ...string) (*template.Template, error) {
	// no need to recreate templates in prod builds
	// since they're not going to change between renders
	var cacheKey string
	if build.IsProd {
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
	if build.IsProd {
		tmplCache.Store(cacheKey, tmpl)
	}
	return tmpl, nil
}

func readTemplate(path string) ([]byte, error) {
	in, err := templates.FS.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s open failed: %w", path, err)
	}
	defer in.Close()

	buf, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("%s read failed: %w", path, err)
	}
	return buf, nil
}
