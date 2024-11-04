package sessions

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func init() {
	gob.Register(time.Time{})
}

func Encode(in map[string]any) ([]byte, error) {
	buf := webapp.BufBorrow()
	defer webapp.BufReturn(buf)

	if err := gob.NewEncoder(buf).Encode(in); err != nil {
		return nil, err
	}
	return bytes.Clone(buf.Bytes()), nil
}

func Decode(in []byte) (map[string]any, error) {
	var out map[string]any
	err := gob.NewDecoder(bytes.NewReader(in)).Decode(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
