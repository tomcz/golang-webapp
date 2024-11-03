//go:build prod

package static

import (
	"embed"
	"net/http"
)

//go:embed *.js
//go:embed lib/*
var content embed.FS

// FS provides a filesystem of static assets
// that are embedded into the production binary.
var FS http.FileSystem = http.FS(content)
