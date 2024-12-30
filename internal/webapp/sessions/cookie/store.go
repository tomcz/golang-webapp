package cookie

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
	"github.com/tink-crypto/tink-go/v2/tink"

	"github.com/tomcz/golang-webapp/internal/webapp"
	"github.com/tomcz/golang-webapp/internal/webapp/sessions"
)

const (
	sessionExpiresAt   = "_Expires_At_"
	compressThreshold  = 1024 // bytes
	compressedPrefix   = "c."
	uncompressedPrefix = "u."
)

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

	data, err := sessions.Encode(session)
	if err != nil {
		return "", err
	}
	return s.encrypt(data)
}

func (s *cookieStore) Read(value string) (map[string]any, error) {
	if value == "" {
		return nil, errors.New("nothing to read")
	}
	data, err := s.decrypt(value)
	if err != nil {
		return nil, err
	}
	session, err := sessions.Decode(data)
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

func (s *cookieStore) encrypt(data []byte) (string, error) {
	if len(data) > compressThreshold {
		return s.compressAndEncrypt(data)
	}
	return s.encryptAndEncode(data, uncompressedPrefix)
}

func (s *cookieStore) decrypt(value string) ([]byte, error) {
	if strings.HasPrefix(value, compressedPrefix) {
		return s.decryptAndUncompress(value)
	}
	if strings.HasPrefix(value, uncompressedPrefix) {
		return s.decodeAndDecrypt(value, uncompressedPrefix)
	}
	return nil, errors.New("invalid value prefix")
}

func (s *cookieStore) encryptAndEncode(plainText []byte, prefix string) (string, error) {
	cipherText, err := s.cipher.Encrypt(plainText, nil)
	if err != nil {
		return "", err
	}
	encoded := base64.URLEncoding.EncodeToString(cipherText)
	return prefix + encoded, nil
}

func (s *cookieStore) decodeAndDecrypt(value string, prefix string) ([]byte, error) {
	cipherText, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, err
	}
	return s.cipher.Decrypt(cipherText, nil)
}

func (s *cookieStore) compressAndEncrypt(plainText []byte) (string, error) {
	buf := webapp.BufBorrow()
	defer webapp.BufReturn(buf)

	w := gzip.NewWriter(buf)
	_, err := w.Write(plainText)
	if err != nil {
		return "", err
	}
	if err = w.Close(); err != nil {
		return "", err
	}
	return s.encryptAndEncode(buf.Bytes(), compressedPrefix)
}

func (s *cookieStore) decryptAndUncompress(value string) ([]byte, error) {
	plainText, err := s.decodeAndDecrypt(value, compressedPrefix)
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewReader(plainText))
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err = r.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}
