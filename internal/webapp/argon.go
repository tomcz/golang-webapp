package webapp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var hashEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// HashPassword returns a salted & hashed representation of the password.
func HashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	hash := hashPassword([]byte(password), salt)
	return hashEncoding.EncodeToString(salt) + "." + hashEncoding.EncodeToString(hash)
}

// MatchPassword returns true if the password matches the hashed password.
// The hashed password is expected to be the output of [HashPassword].
func MatchPassword(hashedPassword, password string) (bool, error) {
	before, after, found := strings.Cut(hashedPassword, ".")
	if !found {
		return false, errors.New("invalid hashed password")
	}
	salt, err := hashEncoding.DecodeString(before)
	if err != nil {
		return false, fmt.Errorf("invalid salt part: %w", err)
	}
	expectedHash, err := hashEncoding.DecodeString(after)
	if err != nil {
		return false, fmt.Errorf("invalid hash part: %w", err)
	}
	actualHash := hashPassword([]byte(password), salt)
	res := subtle.ConstantTimeCompare(expectedHash, actualHash)
	return res != 0, nil
}

func hashPassword(password []byte, salt []byte) []byte {
	return argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
}
