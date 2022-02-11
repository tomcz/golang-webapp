package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/oxtoacart/bpool"
	log "github.com/sirupsen/logrus"

	"github.com/tomcz/golang-webapp/templates"
)

// use a buffer pool to prevent rendering partial
// content when we fail to evaluate a template
var bufPool = bpool.NewBufferPool(48)

func render(w http.ResponseWriter, r *http.Request, data map[string]interface{}, templatePaths ...string) {
	if !saveSession(w, r) {
		return
	}
	ll := log.WithField("paths", templatePaths)
	tmpl, err := newTemplate(templatePaths)
	if err != nil {
		ll.WithError(err).Error("failed to create template")
		http.Error(w, "failed to create template", http.StatusInternalServerError)
		return
	}
	buf := bufPool.Get()
	defer bufPool.Put(buf)
	err = tmpl.ExecuteTemplate(buf, "main", data)
	if err != nil {
		ll.WithError(err).Error("failed to evaluate template")
		http.Error(w, "failed to evaluate template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = buf.WriteTo(w)
	if err != nil {
		ll.WithError(err).Error("failed to write template")
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
