package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	require.Contains(t, cached, "https://go.dev/blog/pipelines")

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

// เทสต์ expiry เดิมยิงเฉพาะตอน cache เย็น ซึ่งไม่ครอบเคสจริง: ลิงก์ที่ถูก cache
// ไว้ตอนยังไม่หมดอายุ แล้วหมดอายุระหว่างที่ entry ยังอยู่ใน Redis
func TestALinkThatExpiresWhileCachedStopsRedirecting(t *testing.T) {
	e := newEnv(t)
	soon := time.Now().Add(1500 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	_, created := createLink(t, e.app, `{"long_url":"https://go.dev/","expires_at":"`+soon+`"}`)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode, "still valid, should redirect")
	require.NoError(t, e.rdb.Get(t.Context(), "link:"+created.Code).Err(), "cache should be warm")

	time.Sleep(1800 * time.Millisecond)

	res, err = e.app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode,
		"an expired link must not keep redirecting just because it is still cached")
}
