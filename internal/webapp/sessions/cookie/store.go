package cookie

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"github.com/tink-crypto/tink-go/v2/tink"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

const sessionExpiresAt = "_Expires_At_"

type cookieStore struct {
	cipher  tink.AEAD
	timeNow func() time.Time
}

func New(sessionKey string) (webapp.SessionStore, error) {
	keyBytes, err := keyToBytes(sessionKey)
	if err != nil {
		return nil, err
	}
	cipher, err := subtle.NewAESGCMSIV(keyBytes)
	if err != nil {
		return nil, err
	}
	return &cookieStore{
		cipher:  cipher,
		timeNow: time.Now,
	}, nil
}

func keyToBytes(key string) ([]byte, error) {
	if key == "" {
		return sessions.NewKeyBytes(), nil
	}
	if err := sessions.ValidateKey(key); err != nil {
		return nil, err
	}
	return sessions.DecodeKey(key)
}

func (s *cookieStore) Write(_ string, session map[string]any, maxAge time.Duration) (string, error) {
	session[sessionExpiresAt] = s.timeNow().Add(maxAge)
	defer func() {
		delete(session, sessionExpiresAt)
	}()

	plainText, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}

	cipherText, err := s.cipher.Encrypt(plainText, nil)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(cipherText), nil
}

func (s *cookieStore) Read(value string) (map[string]any, error) {
	if value == "" {
		return nil, errors.New("nothing to decode")
	}

	cipherText, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}

	plainText, err := s.cipher.Decrypt(cipherText, nil)
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
	if expiresAt.Before(s.timeNow()) {
		return nil, errors.New("session has expired")
	}

	delete(session, sessionExpiresAt)
	return session, nil
}

func (s *cookieStore) Delete(string) {
	// No backend to purge cookie data from
}

func (s *cookieStore) Close() error {
	return nil // No backend to disconnect
}
