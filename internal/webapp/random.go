package webapp

import (
	"encoding/hex"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
)

func shortRandomText() string {
	return hex.EncodeToString(random.GetRandomBytes(8))
}

func longRandomText() string {
	return hex.EncodeToString(random.GetRandomBytes(32))
}
