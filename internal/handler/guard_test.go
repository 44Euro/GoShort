package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"goshort/internal/middleware"
)

// ไล่ทุก route ที่ลงทะเบียนไว้จริง ไม่ใช่รายการที่พิมพ์มือไว้ซึ่งจะตกหล่นเมื่อเพิ่มเส้นใหม่
func TestEveryAdminRouteRefusesAnAnonymousCaller(t *testing.T) {
	app := newApp(t)

	checked := 0
	for _, r := range app.GetRoutes() {
		if !strings.HasPrefix(r.Path, "/api/admin") || r.Method == "HEAD" {
			continue
		}
		if r.Path == "/api/admin/login" || r.Path == "/api/admin/logout" {
			continue
		}
		path := strings.ReplaceAll(r.Path, ":code", "gopher")

		res, err := app.Test(httptest.NewRequest(r.Method, path, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, res.StatusCode,
			"%s %s is reachable without a session", r.Method, r.Path)
		checked++
	}
	require.Greater(t, checked, 0, "no admin routes were checked")
}

func TestTamperedAndExpiredSessionsAreRefused(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t, "admin@goshort.dev", "goshort-demo")
	good := sessionCookie(t, login(t, e.app, "admin@goshort.dev", "goshort-demo"))

	wrongSecret, _, err := middleware.IssueToken("not-the-real-secret", "admin@goshort.dev")
	require.NoError(t, err)

	expired := signedWith(t, "test-secret", jwt.RegisteredClaims{
		Subject:   "admin@goshort.dev",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})

	// alg=none คือการโจมตี JWT คลาสสิก ต้องถูกปฏิเสธแม้ payload จะถูกทุกอย่าง
	algNone, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   "admin@goshort.dev",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	for name, token := range map[string]string{
		"signed with another secret": wrongSecret,
		"expired":                    expired,
		"alg none":                   algNone,
		"garbage":                    "not-a-jwt",
		"empty":                      "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookie, Value: token})
		res, err := e.app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, res.StatusCode, "token %q must be refused", name)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.AddCookie(good)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode, "the genuine cookie should still work")
}

func signedWith(t *testing.T, secret string, claims jwt.RegisteredClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}
