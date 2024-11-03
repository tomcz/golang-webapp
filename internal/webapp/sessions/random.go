package sessions

import (
	"encoding/hex"
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

func ValidKey(key string) bool {
	return validKeyPattern.MatchString(key)
}

func KeyBytes(key string) ([]byte, error) {
	return hex.DecodeString(key)
}
