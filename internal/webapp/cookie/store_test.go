package cookie

import (
	"context"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	clocks "k8s.io/utils/clock/testing"
)

func TestCodecRoundTrip(t *testing.T) {
	now := time.Now()
	clock := clocks.NewFakePassiveClock(now)

	store := &cookieStore{
		key:   randomKey(),
		clock: clock,
	}

	data := map[string]any{"wibble": "wobble"}

	encoded, err := store.Encode(context.Background(), data, time.Hour)
	assert.NilError(t, err)

	clock.SetTime(now.Add(time.Minute))

	decoded, err := store.Decode(context.Background(), encoded)
	assert.NilError(t, err)

	assert.DeepEqual(t, data, decoded)
}

func TestCodecRoundTrip_Expired(t *testing.T) {
	now := time.Now()
	clock := clocks.NewFakePassiveClock(now)

	store := &cookieStore{
		key:   randomKey(),
		clock: clock,
	}

	data := map[string]any{"wibble": "wobble"}

	encoded, err := store.Encode(context.Background(), data, time.Minute)
	assert.NilError(t, err)

	clock.SetTime(now.Add(time.Hour))

	_, err = store.Decode(context.Background(), encoded)
	assert.ErrorContains(t, err, "session expired")
}
