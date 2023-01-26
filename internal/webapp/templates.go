package webapp

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/oxtoacart/bpool"

	"github.com/tomcz/golang-webapp/build"
	"github.com/tomcz/golang-webapp/templates"
)

// use a buffer pool to avoid creating and freeing
// buffers when we execute templates to avoid writing
// incomplete or malformed data to the response
var bufPool = bpool.NewBufferPool(48)

// no need to recreate templates in prod builds
// since they're not going to change between renders
var tmplCache = make(map[string]*template.Template)
var tmplLock sync.RWMutex

func RenderErr(w http.ResponseWriter, r *http.Request, err error, msg string, statusCode int) {
	rerr(r, err)
	if id := rid(r); id != "" {
		msg = fmt.Sprintf("ID: %s\nError: %s\n", id, msg)
	}
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
		RenderErr(w, r, err, "Failed to create template", http.StatusInternalServerError)
		return
	}
	buf := bufPool.Get()
	defer bufPool.Put(buf)
	err = tmpl.ExecuteTemplate(buf, "main", data)
	if err != nil {
		err = fmt.Errorf("template exec: %w", err)
		RenderErr(w, r, err, "Failed to execute template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = buf.WriteTo(w)
	if err != nil {
		rerr(r, fmt.Errorf("template write: %w", err))
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