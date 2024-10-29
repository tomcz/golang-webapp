package cookie

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"github.com/tink-crypto/tink-go/v2/subtle/random"
	"k8s.io/utils/clock"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

const sessionExpiresAt = "_Expires_At_"

func init() {
	gob.Register(time.Time{})
}

func RandomKey() string {
	return base64.StdEncoding.EncodeToString(randomKey())
}

func randomKey() []byte {
	return random.GetRandomBytes(32)
}

func keyToBytes(key string) ([]byte, error) {
	if key == "" {
		return randomKey(), nil
	}
	buf, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("bad key: %w", err)
	}
	if len(buf) != 32 {
		return nil, fmt.Errorf("expected 32-byte key, got %d bytes", len(buf))
	}
	return buf, nil
}

type cookieStore struct {
	key   []byte
	clock clock.PassiveClock
}

func New(sessionKey string) (webapp.SessionCodec, error) {
	keyBytes, err := keyToBytes(sessionKey)
	if err != nil {
		return nil, err
	}
	return &cookieStore{
		key:   keyBytes,
		clock: clock.RealClock{},
	}, nil
}

func (c *cookieStore) Close() error {
	return nil
}

func (c *cookieStore) Encode(_ context.Context, session map[string]any, maxAge time.Duration) (string, error) {
	session[sessionExpiresAt] = c.clock.Now().Add(maxAge)
	defer func() {
		delete(session, sessionExpiresAt)
	}()

	buf := webapp.BufBorrow()
	defer webapp.BufReturn(buf)

	err := gob.NewEncoder(buf).Encode(session)
	if err != nil {
		return "", fmt.Errorf("gob.encode: %w", err)
	}

	cipher, err := subtle.NewAESGCMSIV(c.key)
	if err != nil {
		return "", fmt.Errorf("cipher.new: %w", err)
	}

	cipherText, err := cipher.Encrypt(buf.Bytes(), nil)
	if err != nil {
		return "", fmt.Errorf("cipher.encrypt: %w", err)
	}

	return base64.URLEncoding.EncodeToString(cipherText), nil
}

func (c *cookieStore) Decode(_ context.Context, value string) (map[string]any, error) {
	cipherText, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("value.decode: %w", err)
	}

	cipher, err := subtle.NewAESGCMSIV(c.key)
	if err != nil {
		return nil, fmt.Errorf("cipher.new: %w", err)
	}

	plainText, err := cipher.Decrypt(cipherText, nil)
	if err != nil {
		return nil, fmt.Errorf("cipher.decrypt: %w", err)
	}

	var session map[string]any
	err = gob.NewDecoder(bytes.NewReader(plainText)).Decode(&session)
	if err != nil {
		return nil, fmt.Errorf("gob.decode: %w", err)
	}

	expiresTxt, ok := session[sessionExpiresAt]
	if !ok {
		return nil, fmt.Errorf("no session expiry")
	}
	expiresAt, ok := expiresTxt.(time.Time)
	if !ok {
		return nil, fmt.Errorf("session expiry is not a time")
	}
	if expiresAt.Before(c.clock.Now()) {
		return nil, fmt.Errorf("session expired at %s", expiresAt)
	}

	delete(session, sessionExpiresAt)
	return session, nil
}
