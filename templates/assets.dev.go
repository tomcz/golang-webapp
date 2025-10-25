//go:build !prod

package templates

import "net/http"

var Embedded = false

// FS provides a filesystem of html templates
// so that you can edit them without restarting the app.
var FS http.FileSystem = http.Dir("templates")
