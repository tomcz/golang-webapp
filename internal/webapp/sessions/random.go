package sessions

import (
	"encoding/hex"
	"errors"
	"regexp"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
)

var validKeyPattern = regexp.MustCompile(`^[[:xdigit:]]{64}$`)

func RandomKey() string {
	return hex.EncodeToString(RandomBytes())
}

func RandomBytes() []byte {
	return random.GetRandomBytes(32)
}

func ValidKey(key string) error {
	if key == "" {
		return errors.New("empty key")
	}
	if validKeyPattern.MatchString(key) {
		return nil
	}
	return errors.New("invalid key")
}

func KeyBytes(key string) ([]byte, error) {
	return hex.DecodeString(key)
}
