package cookie

import (
	"testing"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"gotest.tools/v3/assert"

	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

func TestRoundTrip(t *testing.T) {
	cipher, err := subtle.NewAESGCMSIV(sessions.NewKeyBytes())
	assert.NilError(t, err)

	now := time.Now()
	store := &cookieStore{
		cipher:  cipher,
		timeNow: func() time.Time { return now },
	}

	data := map[string]any{"wibble": "wobble"}

	encoded, err := store.Write("", data, time.Hour)
	assert.NilError(t, err)

	now = now.Add(time.Minute)

	decoded, err := store.Read(encoded)
	assert.NilError(t, err)

	assert.DeepEqual(t, data, decoded)
}

func TestRoundTrip_Expired(t *testing.T) {
	cipher, err := subtle.NewAESGCMSIV(sessions.NewKeyBytes())
	assert.NilError(t, err)

	now := time.Now()
	store := &cookieStore{
		cipher:  cipher,
		timeNow: func() time.Time { return now },
	}

	data := map[string]any{"wibble": "wobble"}

	encoded, err := store.Write("", data, time.Minute)
	assert.NilError(t, err)

	now = now.Add(time.Hour)

	_, err = store.Read(encoded)
	assert.ErrorContains(t, err, "session has expired")
}
