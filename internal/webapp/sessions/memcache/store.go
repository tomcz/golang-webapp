package memcache

import (
	"io"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

const thirtyDaysInSeconds = 60 * 60 * 24 * 30

// for replacement in tests
type memcachedClient interface {
	Set(item *memcache.Item) error
	Get(key string) (*memcache.Item, error)
	Delete(key string) error
	io.Closer
}

type memcacheStore struct {
	mdb memcachedClient
}

func New(addr string) webapp.SessionStore {
	return &memcacheStore{
		mdb: memcache.New(addr),
	}
}

func (s *memcacheStore) Write(key string, session map[string]any, maxAge time.Duration) (string, error) {
	encoded, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}
	if sessions.ValidateKey(key) != nil {
		key = sessions.NewKey()
	}
	// Ref: https://github.com/memcached/memcached/wiki/Programming#expiration
	// Expiration times are specified in unsigned integer seconds.
	// They can be set from 0, meaning "never expire", to 30 days (60*60*24*30).
	// Any time higher than 30 days is interpreted as a unix timestamp date.
	// If you want to expire an object on january 1st of next year, this is how you do that.
	maxAgeInSeconds := int32(maxAge.Seconds())
	if maxAgeInSeconds > thirtyDaysInSeconds {
		maxAgeInSeconds = int32(time.Now().Add(maxAge).Unix())
	}
	err = s.mdb.Set(&memcache.Item{
		Key:        key,
		Value:      encoded,
		Expiration: maxAgeInSeconds,
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *memcacheStore) Read(key string) (map[string]any, error) {
	if err := sessions.ValidateKey(key); err != nil {
		return nil, err
	}
	item, err := s.mdb.Get(key)
	if err != nil {
		return nil, err
	}
	return sessions.Decode(item.Value)
}

func (s *memcacheStore) Delete(key string) {
	if sessions.ValidateKey(key) == nil {
		_ = s.mdb.Delete(key)
	}
}

func (s *memcacheStore) Close() error {
	return s.mdb.Close()
}
