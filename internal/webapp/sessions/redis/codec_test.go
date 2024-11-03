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

	codec := &redisCodec{rdb}
	ctx := context.Background()
	data1 := map[string]any{"wibble": "wobble"}
	data2 := map[string]any{"wibble": "waggle"}

	key1, err := codec.Encode(ctx, "", data1, time.Hour)
	assert.NilError(t, err)

	decoded, err := codec.Decode(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data1, decoded)

	key2, err := codec.Encode(ctx, key1, data2, time.Hour)
	assert.NilError(t, err)
	assert.Equal(t, key1, key2)

	decoded, err = codec.Decode(ctx, key1)
	assert.NilError(t, err)
	assert.DeepEqual(t, data2, decoded)

	codec.Clear(ctx, key1)

	_, err = codec.Decode(ctx, key1)
	assert.ErrorIs(t, err, redis.Nil)
}
