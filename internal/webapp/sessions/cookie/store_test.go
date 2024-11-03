package cookie

import (
	"testing"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"gotest.tools/v3/assert"
	clocks "k8s.io/utils/clock/testing"

	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

func TestRoundTrip(t *testing.T) {
	now := time.Now()
	clock := clocks.NewFakePassiveClock(now)

	cipher, err := subtle.NewAESGCMSIV(sessions.RandomBytes())
	assert.NilError(t, err)

	store := &cookieStore{
		cipher: cipher,
		clock:  clock,
	}

	data := map[string]any{"wibble": "wobble"}

	encoded, err := store.Write("", data, time.Hour)
	assert.NilError(t, err)

	clock.SetTime(now.Add(time.Minute))

	decoded, err := store.Read(encoded)
	assert.NilError(t, err)

	assert.DeepEqual(t, data, decoded)
}

func TestRoundTrip_Expired(t *testing.T) {
	now := time.Now()
	clock := clocks.NewFakePassiveClock(now)

	cipher, err := subtle.NewAESGCMSIV(sessions.RandomBytes())
	assert.NilError(t, err)

	store := &cookieStore{
		cipher: cipher,
		clock:  clock,
	}

	data := map[string]any{"wibble": "wobble"}

	encoded, err := store.Write("", data, time.Minute)
	assert.NilError(t, err)

	clock.SetTime(now.Add(time.Hour))

	_, err = store.Read(encoded)
	assert.ErrorContains(t, err, "session has expired")
}
