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
	ctx := context.Background()
	data := map[string]any{"wibble": "wobble"}

	key1, err := store.Encode(ctx, "", data, time.Hour)
	assert.NilError(t, err)

	decoded, err := store.Decode(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data, decoded)

	key2, err := store.Encode(ctx, key1, data, time.Hour)
	assert.NilError(t, err)
	assert.Equal(t, key1, key2)

	store.Clear(ctx, key1)

	_, err = store.Decode(ctx, key1)
	assert.ErrorIs(t, err, redis.Nil)
}
