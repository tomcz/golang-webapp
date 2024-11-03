package memory

import (
	"errors"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

type memoryStore struct {
	cache *cache.Cache
}

func New() webapp.SessionStore {
	return &memoryStore{
		cache: cache.New(time.Hour, 10*time.Minute),
	}
}

func (s *memoryStore) Write(key string, session map[string]any, maxAge time.Duration) (string, error) {
	encoded, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}
	if !sessions.ValidKey(key) {
		key = sessions.RandomKey()
	}
	s.cache.Set(key, encoded, maxAge)
	return key, nil
}

func (s *memoryStore) Read(key string) (map[string]any, error) {
	if !sessions.ValidKey(key) {
		return nil, errors.New("invalid key")
	}
	encoded, ok := s.cache.Get(key)
	if !ok {
		return nil, errors.New("key not found")
	}
	return sessions.Decode(encoded.([]byte))
}

func (s *memoryStore) Delete(key string) {
	if sessions.ValidKey(key) {
		s.cache.Delete(key)
	}
}

func (s *memoryStore) Close() error {
	return nil
}
