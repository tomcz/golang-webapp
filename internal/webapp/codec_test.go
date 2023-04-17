package webapp

import (
	"crypto/rand"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestCodecRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	assert.NilError(t, err)

	codec := &sessionCodec{
		name:   "test",
		key:    key,
		maxAge: 24 * time.Hour,
		path:   "/",
	}

	now := time.Now()
	data := map[string]any{"wibble": "wobble"}

	encoded, err := codec.encode(data, now.Add(codec.maxAge))
	assert.NilError(t, err)

	decoded, err := codec.decode(encoded, now.Add(time.Hour))
	assert.NilError(t, err)

	assert.DeepEqual(t, data, decoded)
}
