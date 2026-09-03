package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func login(t *testing.T, app *fiber.App, email, password string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login",
		strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	return res
}

func sessionCookie(t *testing.T, res *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range res.Cookies() {
		if c.Name == "goshort_session" {
			return c
		}
	}
	t.Fatal("no session cookie in response")
	return nil
}

func TestLoginSetsAnHttpOnlySessionCookie(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t, "admin@goshort.dev", "goshort-demo")

	res := login(t, e.app, "admin@goshort.dev", "goshort-demo")
	require.Equal(t, http.StatusOK, res.StatusCode)

	c := sessionCookie(t, res)
	require.True(t, c.HttpOnly, "page JavaScript must not be able to read the session token")
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.NotEmpty(t, c.Value)
}

func TestLoginFailureDoesNotRevealWhichHalfWasWrong(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t, "admin@goshort.dev", "goshort-demo")

	wrongPass := login(t, e.app, "admin@goshort.dev", "nope")
	wrongUser := login(t, e.app, "nobody@goshort.dev", "goshort-demo")

	require.Equal(t, http.StatusUnauthorized, wrongPass.StatusCode)
	require.Equal(t, http.StatusUnauthorized, wrongUser.StatusCode)
	require.Equal(t, readBody(t, wrongPass), readBody(t, wrongUser))
}

func TestMeReportsTheSignedInAdmin(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t, "admin@goshort.dev", "goshort-demo")
	c := sessionCookie(t, login(t, e.app, "admin@goshort.dev", "goshort-demo"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.AddCookie(c)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Contains(t, readBody(t, res), "admin@goshort.dev")
}

func TestMeWithoutACookieIsUnauthorized(t *testing.T) {
	app := newApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/admin/me", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestLogoutClearsTheCookie(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t, "admin@goshort.dev", "goshort-demo")
	c := sessionCookie(t, login(t, e.app, "admin@goshort.dev", "goshort-demo"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	req.AddCookie(c)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Empty(t, sessionCookie(t, res).Value, "logout must blank the cookie server side")
}
