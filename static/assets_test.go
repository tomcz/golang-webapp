//go:build !dev

package static

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestEmbedding(t *testing.T) {
	assert.Assert(t, Embedded)
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			fp, fail := FS.Open(path)
			assert.NilError(t, fail)
			assert.NilError(t, fp.Close())
		})
		return nil
	})
	assert.NilError(t, err)
}
