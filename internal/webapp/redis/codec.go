package redis

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

var validKeyPattern = regexp.MustCompile(`^[[:xdigit:]]{64}$`)

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
	buf := webapp.BufBorrow()
	defer webapp.BufReturn(buf)

	if err := gob.NewEncoder(buf).Encode(session); err != nil {
		return "", fmt.Errorf("gob.Encode: %w", err)
	}
	value := base64.StdEncoding.EncodeToString(buf.Bytes())

	if !validKeyPattern.MatchString(key) {
		key = fmt.Sprintf("%x", random.GetRandomBytes(32))
	}

	if err := c.rdb.Set(ctx, key, value, maxAge).Err(); err != nil {
		return "", fmt.Errorf("redis.Set: %w", err)
	}
	return key, nil
}

func (c *redisCodec) Decode(ctx context.Context, key string) (map[string]any, error) {
	if !validKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("invalid redis key %q", key)
	}

	value, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis.Get: %w", err)
	}

	buf, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("base64.Decode: %w", err)
	}

	var session map[string]any
	err = gob.NewDecoder(bytes.NewReader(buf)).Decode(&session)
	if err != nil {
		return nil, fmt.Errorf("gob.Decode: %w", err)
	}
	return session, nil
}

func (c *redisCodec) Clear(ctx context.Context, key string) {
	if validKeyPattern.MatchString(key) {
		c.rdb.Del(ctx, key)
	}
}

func (c *redisCodec) Close() error {
	return c.rdb.Close()
}
