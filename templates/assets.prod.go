//go:build prod

package templates

import (
	"embed"
	"net/http"
)

//go:embed *.gohtml
var content embed.FS

var Embedded = true

// FS provides a filesystem of html templates
// that are embedded into the production binary.
var FS http.FileSystem = http.FS(content)
