package cookie

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestCodecRoundTrip(t *testing.T) {
	key, err := randomKey()
	assert.NilError(t, err)

	codec := &cookieStore{
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

	codec := &cookieStore{
		name:   "test",
		key:    key,
		maxAge: 24 * time.Hour,
		path:   "/",
	}

	data := map[string]any{"wibble": "wobble"}
	outReq := httptest.NewRequest(http.MethodGet, "https://example.com/foo", nil)
	outRes := httptest.NewRecorder()
	err = codec.SetSession(outRes, outReq, data)
	assert.NilError(t, err)

	cookies := outRes.Result().Cookies()
	idx := slices.IndexFunc(cookies, func(c *http.Cookie) bool { return c.Name == codec.name })
	assert.Assert(t, idx >= 0)
	cookie := cookies[idx]
	assert.Equal(t, codec.path, cookie.Path)
	assert.Equal(t, int(codec.maxAge.Seconds()), cookie.MaxAge)
	assert.Equal(t, true, cookie.Secure)
	assert.Equal(t, true, cookie.HttpOnly)

	inReq := httptest.NewRequest(http.MethodGet, "/bar", nil)
	inReq.Header.Set("Cookie", cookie.String())
	actual, err := codec.GetSession(inReq)
	assert.NilError(t, err)

	assert.DeepEqual(t, data, actual)
}
