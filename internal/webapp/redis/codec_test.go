package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gotest.tools/v3/assert"
)

func TestCodecRoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NilError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	store := &redisCodec{rdb}
	data := map[string]any{"wibble": "wobble"}

	key, err := store.Encode(context.Background(), data, time.Hour)
	assert.NilError(t, err)

	decoded, err := store.Decode(context.Background(), key)
	assert.NilError(t, err)

	assert.Equal(t, "wobble", decoded["wibble"])
	assert.Equal(t, key, decoded[redisKeyName])
}
