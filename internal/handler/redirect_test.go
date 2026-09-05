package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedirectSendsTheVisitorToTheDestination(t *testing.T) {
	app := newApp(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/blog/pipelines"}`)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)

	require.Equal(t, http.StatusFound, res.StatusCode)
	require.Equal(t, "https://go.dev/blog/pipelines", res.Header.Get("Location"))
}

// ปลายทางที่มี query string กับอักขระที่ต้อง encode คือจุดที่ URL เพี้ยนได้ง่ายที่สุด
// และเป็นทางเดียวกับที่ referrer เคยถูก buffer ของ fasthttp เอากลับไปใช้ซ้ำจนสตริงต่อกัน
// ต้องออกมาเหมือนที่ใส่เข้าไปทุก byte ทั้งตอน cache เย็นและตอน cache อุ่น
func TestTheDestinationSurvivesQueryStringsAndEncoding(t *testing.T) {
	app := newApp(t)
	const long = "https://go.dev/blog/pipelines?utm_source=goshort&utm_medium=e2e&q=a%2Fb+c&ref=one/two/three"

	_, created := createLink(t, app, `{"long_url":"`+long+`"}`)

	for _, pass := range []string{"cold cache", "warm cache"} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, res.StatusCode, pass)
		require.Equal(t, long, res.Header.Get("Location"), "the destination came back altered on the %s", pass)
	}
}

func TestUnknownCodeIsNotFound(t *testing.T) {
	app := newApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/nosuch", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestExpiredLinkIsNotFound(t *testing.T) {
	app := newApp(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/","expires_at":"2020-01-01T00:00:00Z"}`)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

// /:code เป็น wildcard ที่ root ถ้าลำดับ route ผิดมันจะกินเส้นพวกนี้ไปหมด
func TestApplicationRoutesAreNotSwallowedByTheRedirectRoute(t *testing.T) {
	app := newApp(t)

	for _, path := range []string{"/health", "/api/links"} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		require.NotEqual(t, http.StatusFound, res.StatusCode, "%s must not be handled as a short code", path)
		require.Empty(t, res.Header.Get("Location"), "%s must not redirect", path)
	}
}
