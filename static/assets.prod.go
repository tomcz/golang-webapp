//go:build prod

package static

import (
	"embed"
	"net/http"
)

//go:embed *.js
//go:embed *.css
var content embed.FS

var Embedded = true

// FS provides a filesystem of static assets
// that are embedded into the production binary.
var FS http.FileSystem = http.FS(content)
