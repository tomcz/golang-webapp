package webapp

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAbsoluteURL(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	assert.Equal(t, "http://example.com/wibble", AbsoluteURL(req, "/wibble"))
}
