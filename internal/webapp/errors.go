package webapp

import (
	"fmt"
	"net/http"
)

func HttpError(w http.ResponseWriter, r *http.Request, statusCode int, msg string, err error) {
	RSet(r, "error", err)
	msg = fmt.Sprintf("ID: %s\nError: %s\n", ReqID(r), msg)
	http.Error(w, msg, statusCode)
}
