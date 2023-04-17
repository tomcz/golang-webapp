package webapp

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"github.com/google/tink/go/aead/subtle"
)

const sessionExpiresAt = "_Expires_At"

func init() {
	gob.Register(time.Time{})
}

func keyToBytes(key string) []byte {
	if key != "" {
		buf, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			buf = []byte(key)
		}
		if len(buf) == 32 {
			return buf
		}
		sum := sha256.Sum256(buf)
		return sum[:]
	}
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf
}

type sessionCodec struct {
	name   string
	encKey []byte
	maxAge time.Duration
	path   string
}

func (c *sessionCodec) setSession(w http.ResponseWriter, r *http.Request, data map[string]any) error {
	expiresAt := time.Now().Add(c.maxAge)
	value, err := c.encode(data, expiresAt)
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

func (c *sessionCodec) encode(data map[string]any, expiresAt time.Time) (string, error) {
	data[sessionExpiresAt] = expiresAt
	defer func() {
		delete(data, sessionExpiresAt)
	}()
	buf := &bytes.Buffer{}
	defer buf.Reset()
	err := gob.NewEncoder(buf).Encode(data)
	if err != nil {
		return "", fmt.Errorf("gob.encode: %w", err)
	}
	cipher, err := subtle.NewAESGCMSIV(c.encKey)
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
	cipher, err := subtle.NewAESGCMSIV(c.encKey)
	if err != nil {
		return nil, fmt.Errorf("cipher.new: %w", err)
	}
	plainText, err := cipher.Decrypt(cipherText, nil)
	if err != nil {
		return nil, fmt.Errorf("cipher.decrypt: %w", err)
	}
	var result map[string]any
	err = gob.NewDecoder(bytes.NewReader(plainText)).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("gob.decode: %w", err)
	}
	value, ok := result[sessionExpiresAt]
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
	delete(result, sessionExpiresAt)
	return result, nil
}
