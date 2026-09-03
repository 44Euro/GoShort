package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func createFrom(t *testing.T, app *fiber.App, ip string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"long_url":"https://go.dev/"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", ip)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	return res
}

func TestCreatingLinksTooFastIsRateLimited(t *testing.T) {
	app := newApp(t)

	for i := 0; i < 20; i++ {
		res := createFrom(t, app, "203.0.113.9")
		require.Equal(t, http.StatusCreated, res.StatusCode, "request %d should still be inside the quota", i+1)
	}

	res := createFrom(t, app, "203.0.113.9")
	require.Equal(t, http.StatusTooManyRequests, res.StatusCode)
	require.NotEmpty(t, res.Header.Get("Retry-After"))
}

func TestRateLimitIsPerAddress(t *testing.T) {
	app := newApp(t)

	for i := 0; i < 20; i++ {
		createFrom(t, app, "203.0.113.9")
	}
	require.Equal(t, http.StatusTooManyRequests, createFrom(t, app, "203.0.113.9").StatusCode)

	require.Equal(t, http.StatusCreated, createFrom(t, app, "198.51.100.4").StatusCode,
		"a different address must not inherit someone else's quota")
}

func TestRedirectsAreNeverRateLimited(t *testing.T) {
	app := newApp(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)

	for i := 0; i < 40; i++ {
		req := httptest.NewRequest(http.MethodGet, "/"+created.Code, nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		res, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, res.StatusCode)
	}
}
