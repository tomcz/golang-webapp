package cookie

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"github.com/tink-crypto/tink-go/v2/tink"
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
		return nil, errors.New("invalid key format")
	}
	buf, err := sessions.KeyBytes(key)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

type cookieCodec struct {
	cipher tink.AEAD
	clock  clock.PassiveClock
}

func New(sessionKey string) (webapp.SessionCodec, error) {
	keyBytes, err := keyToBytes(sessionKey)
	if err != nil {
		return nil, err
	}
	cipher, err := subtle.NewAESGCMSIV(keyBytes)
	if err != nil {
		return nil, err
	}
	return &cookieCodec{
		cipher: cipher,
		clock:  clock.RealClock{},
	}, nil
}

func (c *cookieCodec) Encode(_ context.Context, _ string, session map[string]any, maxAge time.Duration) (string, error) {
	session[sessionExpiresAt] = c.clock.Now().Add(maxAge)
	defer func() {
		delete(session, sessionExpiresAt)
	}()

	plainText, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}

	cipherText, err := c.cipher.Encrypt(plainText, nil)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(cipherText), nil
}

func (c *cookieCodec) Decode(_ context.Context, value string) (map[string]any, error) {
	if value == "" {
		return nil, errors.New("nothing to decode")
	}

	cipherText, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}

	plainText, err := c.cipher.Decrypt(cipherText, nil)
	if err != nil {
		return nil, err
	}

	session, err := sessions.Decode(plainText)
	if err != nil {
		return nil, err
	}

	expiresTxt, ok := session[sessionExpiresAt]
	if !ok {
		return nil, errors.New("no session expiry")
	}
	expiresAt, ok := expiresTxt.(time.Time)
	if !ok {
		return nil, errors.New("session expiry is not a time")
	}
	if expiresAt.Before(c.clock.Now()) {
		return nil, errors.New("session has expired")
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
