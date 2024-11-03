package memory

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestRoundTrip(t *testing.T) {
	store := New()

	data := map[string]any{"wibble": "wobble"}

	key1, err := store.Write("", data, time.Hour)
	assert.NilError(t, err)
	assert.Assert(t, key1 != "")

	stored, err := store.Read(key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, stored)

	key2, err := store.Write(key1, data, time.Hour)
	assert.NilError(t, err)
	assert.Equal(t, key1, key2)

	store.Delete(key1)

	_, err = store.Read(key1)
	assert.Error(t, err, "key not found")
}
