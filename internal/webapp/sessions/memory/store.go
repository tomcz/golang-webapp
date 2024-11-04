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
	if err = sessions.ValidKey(key); err != nil {
		key = sessions.RandomKey()
	}
	s.cache.Set(key, encoded, maxAge)
	return key, nil
}

func (s *memoryStore) Read(key string) (map[string]any, error) {
	if err := sessions.ValidKey(key); err != nil {
		return nil, err
	}
	if entry, found := s.cache.Get(key); found {
		if encoded, ok := entry.([]byte); ok {
			return sessions.Decode(encoded)
		}
	}
	return nil, errors.New("key not found")
}

func (s *memoryStore) Delete(key string) {
	if err := sessions.ValidKey(key); err == nil {
		s.cache.Delete(key)
	}
}

func (s *memoryStore) Close() error {
	return nil
}
