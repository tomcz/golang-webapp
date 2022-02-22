package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/oxtoacart/bpool"

	"github.com/tomcz/golang-webapp/templates"
)

// use a buffer pool to avoid creating and freeing
// buffers when we execute templates to avoid writing
// incomplete or malformed data to the response
var bufPool = bpool.NewBufferPool(48)

var tmplCache = make(map[string]*template.Template)

type renderData map[string]interface{}

func render(w http.ResponseWriter, r *http.Request, data renderData, templatePaths ...string) {
	if !saveSession(w, r) {
		return
	}
	tmpl, err := newTemplate(templatePaths)
	if err != nil {
		renderError(w, r, err, "failed to create template")
		return
	}
	buf := bufPool.Get()
	defer bufPool.Put(buf)
	err = tmpl.ExecuteTemplate(buf, "main", data)
	if err != nil {
		renderError(w, r, err, "failed to execute template")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = buf.WriteTo(w)
	if err != nil {
		recordError(r, err, "failed to write template")
	}
}

func newTemplate(templatePaths []string) (*template.Template, error) {
	cacheKey := strings.Join(templatePaths, ",")
	if tmpl, ok := tmplCache[cacheKey]; ok {
		return tmpl, nil
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
	// no need to recreate templates in prod builds
	// since they're not going to change between renders
	if templates.Embedded {
		tmplCache[cacheKey] = tmpl
	}
	return tmpl, nil
}

func readTemplate(path string) ([]byte, error) {
	in, err := templates.FS.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s open failed: %w", path, err)
	}
	defer in.Close()
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("%s read failed: %w", path, err)
	}
	return buf, nil
}
