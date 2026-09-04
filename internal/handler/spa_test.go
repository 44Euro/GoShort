package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// นี่คือกับดักหลักของการวาง SPA ไว้ที่ root ร่วมกับ GET /:code
// ถ้าลำดับ route ผิดเมื่อไหร่ การกด refresh บน /admin จะกลายเป็นการหา short code
func TestApplicationPathsAreServedBySPANotByTheRedirectHandler(t *testing.T) {
	e := newEnvWithAssets(t)

	for _, path := range []string{"/", "/login", "/s", "/admin", "/admin/analytics", "/admin/links", "/admin/links/gopher"} {
		res, err := e.app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode, "%s should serve the SPA shell", path)
		require.Empty(t, res.Header.Get("Location"), "%s must never redirect", path)
		require.Contains(t, res.Header.Get("Content-Type"), "text/html")
	}
}

func TestStaticAssetsAreNotTreatedAsShortCodes(t *testing.T) {
	e := newEnvWithAssets(t)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/assets/does-not-exist.js", nil))
	require.NoError(t, err)
	require.Empty(t, res.Header.Get("Location"), "asset paths must never redirect")
}

func TestAnUnknownCodeServesTheSPAWithARealNotFoundStatus(t *testing.T) {
	e := newEnvWithAssets(t)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/nosuchcode", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode, "a dead link must report 404, not 200")
	require.Contains(t, res.Header.Get("Content-Type"), "text/html")
	require.Contains(t, readBody(t, res), "<div id=\"root\">")
}

func TestApiPathsStillReturnJsonNotTheSPAShell(t *testing.T) {
	e := newEnvWithAssets(t)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/api/links/nosuch/stats", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	require.Contains(t, res.Header.Get("Content-Type"), "json")
}

func TestARealCodeStillRedirectsEvenThoughTheSPAIsMounted(t *testing.T) {
	e := newEnvWithAssets(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/gopher", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)
	require.Equal(t, "https://go.dev/", res.Header.Get("Location"))
}
