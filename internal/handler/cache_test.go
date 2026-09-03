package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstRedirectWarmsTheCacheAndSecondReadsFromIt(t *testing.T) {
	app, db, rdb := newAppWithDeps(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/blog/pipelines"}`)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)

	cached, err := rdb.Get(t.Context(), "link:"+created.Code).Result()
	require.NoError(t, err, "first redirect should have written the cache")
	require.Equal(t, "https://go.dev/blog/pipelines", cached)

	// ลบแถวทิ้งโดยไม่แตะ cache ถ้า redirect ยังทำงานอยู่แปลว่าอ่านจาก Redis จริง
	require.NoError(t, db.Exec("DELETE FROM links WHERE short_code = ?", created.Code).Error)

	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)
	require.Equal(t, "https://go.dev/blog/pipelines", res.Header.Get("Location"))
}

func TestRedirectStillWorksWhenRedisIsDown(t *testing.T) {
	app, _, rdb := newAppWithDeps(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)

	require.NoError(t, rdb.Close())

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode, "a dead cache must fall back to postgres, not 500")
	require.Equal(t, "https://go.dev/", res.Header.Get("Location"))
}
