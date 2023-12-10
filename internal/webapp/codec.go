package webapp

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"github.com/google/tink/go/aead/subtle"
)

const sessionExpiresAt = "_Expires_At_"

func init() {
	gob.Register(time.Time{})
}

func RandomKey() (string, error) {
	buf, err := randomKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func randomKey() ([]byte, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	return buf, err
}

func keyToBytes(key string) ([]byte, error) {
	if key == "" {
		return randomKey()
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

type sessionCodec struct {
	name   string
	key    []byte
	maxAge time.Duration
	path   string
}

func (c *sessionCodec) setSession(w http.ResponseWriter, r *http.Request, session map[string]any) error {
	if len(session) == 0 {
		cookie := &http.Cookie{
			Name:     c.name,
			Path:     c.path,
			MaxAge:   -1,
			Secure:   r.URL.Scheme == "https",
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
		return nil
	}
	expiresAt := time.Now().Add(c.maxAge)
	value, err := c.encode(session, expiresAt)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:     c.name,
		Value:    value,
		Path:     c.path,
		Expires:  expiresAt,
		MaxAge:   int(c.maxAge.Seconds()),
		Secure:   r.URL.Scheme == "https",
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	return nil
}

func (c *sessionCodec) getSession(r *http.Request) (map[string]any, error) {
	cookie, err := r.Cookie(c.name)
	if err != nil {
		return nil, err
	}
	return c.decode(cookie.Value, time.Now())
}

func (c *sessionCodec) encode(session map[string]any, expiresAt time.Time) (string, error) {
	session[sessionExpiresAt] = expiresAt
	defer func() {
		delete(session, sessionExpiresAt)
	}()
	buf := bufBorrow()
	defer bufReturn(buf)
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

func (c *sessionCodec) decode(cookieValue string, now time.Time) (map[string]any, error) {
	cipherText, err := base64.URLEncoding.DecodeString(cookieValue)
	if err != nil {
		return nil, fmt.Errorf("cookieValue.decode: %w", err)
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
	value, ok := session[sessionExpiresAt]
	if !ok {
		return nil, fmt.Errorf("no session expiry")
	}
	expiresAt, ok := value.(time.Time)
	if !ok {
		return nil, fmt.Errorf("session expiry is not a time")
	}
	if expiresAt.Before(now) {
		return nil, fmt.Errorf("session expired at %s", expiresAt)
	}
	delete(session, sessionExpiresAt)
	return session, nil
}
