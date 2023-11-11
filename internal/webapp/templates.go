package webapp

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/templates"
)

// no need to recreate templates in prod builds
// since they're not going to change between renders
var tmplCache = make(map[string]*template.Template)
var tmplLock sync.RWMutex

func RenderError(w http.ResponseWriter, r *http.Request, err error, msg string, statusCode int) {
	RSet(r, "error", err)
	msg = fmt.Sprintf("ID: %s\nError: %s\n", rid(r), msg)
	http.Error(w, msg, statusCode)
}

func Render(w http.ResponseWriter, r *http.Request, data map[string]any, templatePaths ...string) {
	if data == nil {
		data = map[string]any{}
	}
	for key, value := range getSessionData(r) {
		data[key] = value
	}
	if !saveSession(w, r) {
		return
	}
	tmpl, err := newTemplate(templatePaths)
	if err != nil {
		err = fmt.Errorf("template new: %w", err)
		RenderError(w, r, err, "Failed to create template", http.StatusInternalServerError)
		return
	}
	// add commit info so we can set versioned static paths
	// to prevent browsers using old assets with a new version
	data["Commit"] = build.Commit()
	// buffer template execution to avoid writing
	// incomplete or malformed data to the response
	buf := &bytes.Buffer{}
	defer buf.Reset()
	err = tmpl.ExecuteTemplate(buf, "main", data)
	if err != nil {
		err = fmt.Errorf("template exec: %w", err)
		RenderError(w, r, err, "Failed to execute template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = buf.WriteTo(w)
	if err != nil {
		rlog(r).Error("template write failed", "error", err)
	}
}

func newTemplate(templatePaths []string) (*template.Template, error) {
	var cacheKey string
	if build.IsProd {
		cacheKey = strings.Join(templatePaths, ",")
		tmplLock.RLock()
		tmpl, ok := tmplCache[cacheKey]
		tmplLock.RUnlock()
		if ok {
			return tmpl, nil
		}
	}
	tmpl := template.New("")
	for _, path := range templatePaths {
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
		tmplLock.Lock()
		tmplCache[cacheKey] = tmpl
		tmplLock.Unlock()
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
