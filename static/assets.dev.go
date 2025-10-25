//go:build !prod

package static

import "net/http"

var Embedded = false

// FS provides a filesystem of static assets.
// so that you can edit them without restarting the app.
var FS http.FileSystem = http.Dir("static")
