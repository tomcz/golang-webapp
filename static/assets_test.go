//go:build !dev

package static

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestEmbedding(t *testing.T) {
	// dammit goland, that is what the test is for
	//goland:noinspection GoBoolExpressions
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
			if assert.Check(t, cmp.Nil(fail)) {
				assert.Check(t, fp.Close())
			}
		})
		return nil
	})
	assert.NilError(t, err)
}
