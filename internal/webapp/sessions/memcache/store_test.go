package memcache

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"gotest.tools/v3/assert"
)

type testAdaptor struct {
	cache sync.Map
}

func (p *testAdaptor) Set(item *memcache.Item) error {
	p.cache.Store(item.Key, item)
	return nil
}

func (p *testAdaptor) Get(key string) (*memcache.Item, error) {
	if value, found := p.cache.Load(key); found {
		if item, ok := value.(*memcache.Item); ok {
			if item.Expiration > thirtyDaysInSeconds {
				return nil, errors.New("unexpected expiration")
			}
			return item, nil
		}
	}
	return nil, memcache.ErrCacheMiss
}

func (p *testAdaptor) Delete(key string) error {
	p.cache.Delete(key)
	return nil
}

func (p *testAdaptor) Close() error {
	return nil
}

func TestRoundTrip(t *testing.T) {
	store := &memcacheStore{mdb: &testAdaptor{}}

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
