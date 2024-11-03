package redis

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

type redisCodec struct {
	rdb *redis.Client
}

func New(address, username, password, tlsType string) webapp.SessionCodec {
	rdb := redis.NewClient(&redis.Options{
		Addr:      address,
		Username:  username,
		Password:  password,
		TLSConfig: redisTLS(tlsType),
	})
	return &redisCodec{rdb}
}

func redisTLS(tlsType string) *tls.Config {
	switch tlsType {
	case "on":
		return &tls.Config{MinVersion: tls.VersionTLS12}
	case "insecure":
		return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	default:
		return nil
	}
}

func (c *redisCodec) Encode(ctx context.Context, key string, session map[string]any, maxAge time.Duration) (string, error) {
	buf, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}
	value := base64.StdEncoding.EncodeToString(buf)

	if !sessions.ValidKey(key) {
		key = sessions.RandomKey()
	}

	err = c.rdb.Set(ctx, key, value, maxAge).Err()
	if err != nil {
		return "", err
	}
	return key, nil
}

func (c *redisCodec) Decode(ctx context.Context, key string) (map[string]any, error) {
	if !sessions.ValidKey(key) {
		return nil, errors.New("invalid session key")
	}

	value, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	buf, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return sessions.Decode(buf)
}

func (c *redisCodec) Clear(ctx context.Context, key string) {
	if sessions.ValidKey(key) {
		c.rdb.Del(ctx, key)
	}
}

func (c *redisCodec) Close() error {
	return c.rdb.Close()
}
