package webapp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tomcz/gotools/slices"
	"gotest.tools/v3/assert"
)

func TestCodecRoundTrip(t *testing.T) {
	key, err := randomKey()
	assert.NilError(t, err)

	codec := &sessionCodec{
		name:   "test",
		key:    key,
		maxAge: 24 * time.Hour,
		path:   "/",
	}

	now := time.Now()
	data := map[string]any{"wibble": "wobble"}

	encoded, err := codec.encode(data, now.Add(codec.maxAge))
	assert.NilError(t, err)

	decoded, err := codec.decode(encoded, now.Add(time.Hour))
	assert.NilError(t, err)

	assert.DeepEqual(t, data, decoded)
}

func TestCodecCookie(t *testing.T) {
	key, err := randomKey()
	assert.NilError(t, err)

	codec := &sessionCodec{
		name:   "test",
		key:    key,
		maxAge: 24 * time.Hour,
		path:   "/",
	}

	data := map[string]any{"wibble": "wobble"}
	outReq := httptest.NewRequest(http.MethodGet, "https://example.com/foo", nil)
	outRes := httptest.NewRecorder()
	err = codec.setSession(outRes, outReq, data)
	assert.NilError(t, err)

	cookies := outRes.Result().Cookies()
	cookie := slices.First(cookies, func(cookie *http.Cookie) bool { return cookie.Name == codec.name })
	assert.Assert(t, cookie != nil)
	assert.Equal(t, codec.path, cookie.Path)
	assert.Equal(t, int(codec.maxAge.Seconds()), cookie.MaxAge)
	assert.Equal(t, true, cookie.Secure)
	assert.Equal(t, true, cookie.HttpOnly)

	inReq := httptest.NewRequest(http.MethodGet, "/bar", nil)
	inReq.Header.Set("Cookie", cookie.String())
	actual, err := codec.getSession(inReq)
	assert.NilError(t, err)

	assert.DeepEqual(t, data, actual)
}
