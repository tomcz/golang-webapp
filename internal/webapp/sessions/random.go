package sessions

import (
	"encoding/hex"
	"errors"
	"regexp"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
)

var validKeyPattern = regexp.MustCompile(`^[[:xdigit:]]{64}$`)

func NewKey() string {
	return hex.EncodeToString(NewKeyBytes())
}

func NewKeyBytes() []byte {
	return random.GetRandomBytes(32)
}

func DecodeKey(key string) ([]byte, error) {
	return hex.DecodeString(key)
}

func ValidateKey(key string) error {
	if key == "" {
		return errors.New("empty key")
	}
	if validKeyPattern.MatchString(key) {
		return nil
	}
	return errors.New("invalid key")
}
