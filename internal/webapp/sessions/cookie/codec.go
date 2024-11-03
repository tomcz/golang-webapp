package cookie

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"k8s.io/utils/clock"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

const sessionExpiresAt = "_Expires_At_"

func keyToBytes(key string) ([]byte, error) {
	if key == "" {
		return sessions.RandomBytes(), nil
	}
	if !sessions.ValidKey(key) {
		return nil, fmt.Errorf("invalid key format")
	}
	buf, err := sessions.KeyBytes(key)
	if err != nil {
		return nil, fmt.Errorf("bad key: %w", err)
	}
	return buf, nil
}

type cookieCodec struct {
	key   []byte
	clock clock.PassiveClock
}

func New(sessionKey string) (webapp.SessionCodec, error) {
	keyBytes, err := keyToBytes(sessionKey)
	if err != nil {
		return nil, err
	}
	return &cookieCodec{
		key:   keyBytes,
		clock: clock.RealClock{},
	}, nil
}

func (c *cookieCodec) Encode(_ context.Context, _ string, session map[string]any, maxAge time.Duration) (string, error) {
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

func (c *cookieCodec) Decode(_ context.Context, value string) (map[string]any, error) {
	if value == "" {
		return nil, fmt.Errorf("nothing to decode")
	}

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

func (c *cookieCodec) Clear(context.Context, string) {
	// No backend to purge cookie data from
}

func (c *cookieCodec) Close() error {
	return nil // No backend to disconnect
}
