package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/oxtoacart/bpool"

	"github.com/tomcz/golang-webapp/templates"
)

// use a buffer pool to avoid creating and freeing
// buffers when we execute templates to avoid writing
// incomplete or malformed data to the response
var bufPool = bpool.NewBufferPool(48)

type renderData map[string]interface{}

func render(w http.ResponseWriter, r *http.Request, data renderData, templatePaths ...string) {
	if !saveSession(w, r) {
		return
	}
	tmpl, err := newTemplate(templatePaths)
	if err != nil {
		rset(r, "err.template_new", err)
		render500(w, r, "Failed to create template")
		return
	}
	buf := bufPool.Get()
	defer bufPool.Put(buf)
	err = tmpl.ExecuteTemplate(buf, "main", data)
	if err != nil {
		rset(r, "err.template_exec", err)
		render500(w, r, "Failed to execute template")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = buf.WriteTo(w)
	if err != nil {
		rset(r, "err.template_write", err)
	}
}

func newTemplate(templatePaths []string) (*template.Template, error) {
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
