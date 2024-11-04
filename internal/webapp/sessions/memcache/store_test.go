package memcache

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"gotest.tools/v3/assert"
)

type testClient struct {
	cache sync.Map
}

func (c *testClient) Set(item *memcache.Item) error {
	c.cache.Store(item.Key, item)
	return nil
}

func (c *testClient) Get(key string) (*memcache.Item, error) {
	if value, found := c.cache.Load(key); found {
		if item, ok := value.(*memcache.Item); ok {
			if item.Expiration > thirtyDaysInSeconds {
				return nil, errors.New("unexpected expiration")
			}
			return item, nil
		}
	}
	return nil, memcache.ErrCacheMiss
}

func (c *testClient) Delete(key string) error {
	c.cache.Delete(key)
	return nil
}

func (c *testClient) Close() error {
	return nil
}

func TestRoundTrip(t *testing.T) {
	store := &memcacheStore{mdb: &testClient{}}

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
	assert.ErrorIs(t, err, memcache.ErrCacheMiss)
}
