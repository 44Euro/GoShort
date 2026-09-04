package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"goshort/internal/config"
	"goshort/internal/middleware"
)

func publicRole(c *config.Config) { c.AdminEnabled = false }

// login ไม่มี guard คลุมอยู่ จึงเป็นเส้นเดียวที่แยก "route หายไปแล้ว" ออกจาก
// "route ยังอยู่แต่ถูกป้องกัน" ได้ชัด — 401 แปลว่ายังลงทะเบียนอยู่ 404 แปลว่าหายจริง
func TestTheAdminApiIsAbsentWhenTheRoleIsDisabled(t *testing.T) {
	e := newEnv(t, publicRole)

	res := login(t, e.app, "admin@goshort.dev", "goshort-demo")
	require.Equal(t, http.StatusNotFound, res.StatusCode,
		"a public instance must not have an admin login to attack")

	for _, r := range e.app.GetRoutes() {
		require.False(t, strings.HasPrefix(r.Path, "/api/admin"),
			"%s %s is still registered on a public instance", r.Method, r.Path)
	}
}

func TestAGenuineSessionStillCannotReachAdminWhenTheRoleIsDisabled(t *testing.T) {
	e := newEnv(t, publicRole)

	token, _, err := middleware.IssueToken(testConfig().JWTSecret, "admin@goshort.dev")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookie, Value: token})
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode,
		"a valid token must not resurrect a route that was never registered")
}

func TestAdminPagesAreNotServedWhenTheRoleIsDisabled(t *testing.T) {
	e := newEnvWithAssets(t, publicRole)

	for _, path := range []string{"/login", "/admin", "/admin/analytics", "/admin/links", "/admin/links/gopher"} {
		res, err := e.app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, res.StatusCode,
			"%s must not exist on a public instance", path)
	}
}

func TestThePublicSurfaceIsUntouchedByTheRoleFlag(t *testing.T) {
	e := newEnvWithAssets(t, publicRole)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/gopher", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)
	require.Equal(t, "https://go.dev/", res.Header.Get("Location"))

	for _, path := range []string{"/", "/s", "/s/gopher", "/health", "/metrics", "/api/stats/public"} {
		res, err := e.app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode,
			"%s must still work on a public instance", path)
	}
}
