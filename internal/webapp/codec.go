package webapp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"time"
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
	sw := &bytes.Buffer{}
	err := gob.NewEncoder(sw).Encode(data)
	if err != nil {
		return "", fmt.Errorf("gob.encode: %w", err)
	}
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return "", fmt.Errorf("aes.new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm.new: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return "", fmt.Errorf("nonce.new: %w", err)
	}
	cipherText := gcm.Seal(nil, nonce, sw.Bytes(), nil)
	value := base64.URLEncoding.EncodeToString(cipherText) + "." + base64.URLEncoding.EncodeToString(nonce)
	return value, nil
}

func (c *sessionCodec) decode(cookieValue string, now time.Time) (map[string]any, error) {
	tokens := strings.SplitN(cookieValue, ".", 2)
	if len(tokens) != 2 {
		return nil, fmt.Errorf("expected 2 tokens, got %d", len(tokens))
	}
	cipherText, err := base64.URLEncoding.DecodeString(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("cipherText.decode: %w", err)
	}
	nonce, err := base64.URLEncoding.DecodeString(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("nonce.decode: %w", err)
	}
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return nil, fmt.Errorf("aes.new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm.new: %w", err)
	}
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.open: %w", err)
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
