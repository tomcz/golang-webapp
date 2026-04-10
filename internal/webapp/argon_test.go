package webapp

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestMatchPassword_Match(t *testing.T) {
	const password = "letmein"
	hash := HashPassword(password)
	match, err := MatchPassword(hash, password)
	assert.NilError(t, err)
	assert.Assert(t, match)
}

func TestMatchPassword_NoMatch(t *testing.T) {
	hash := HashPassword("letmein")
	match, err := MatchPassword(hash, "password0")
	assert.NilError(t, err)
	assert.Assert(t, !match)
}
