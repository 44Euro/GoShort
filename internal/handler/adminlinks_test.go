package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func (e env) adminGet(t *testing.T, path string) *http.Response {
	t.Helper()
	return e.adminReq(t, http.MethodGet, path)
}

func (e env) adminReq(t *testing.T, method, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(e.admin(t))
	res, err := e.app.Test(req, -1)
	require.NoError(t, err)
	return res
}

func TestAdminSeesEveryLinkWithItsCountsAndStatus(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)
	createLink(t, e.app, `{"long_url":"https://k6.io/","alias":"k6-run","expires_at":"2020-01-01T00:00:00Z"}`)

	res := e.adminGet(t, "/api/admin/links")
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		Links []struct {
			Code    string  `json:"code"`
			LongURL string  `json:"long_url"`
			Clicks  int64   `json:"clicks"`
			Status  string  `json:"status"`
			Last14  []int64 `json:"last_14_days"`
			Created string  `json:"created_at"`
		} `json:"links"`
	}
	decode(t, res, &body)

	require.Len(t, body.Links, 2)
	byCode := map[string]string{}
	for _, l := range body.Links {
		byCode[l.Code] = l.Status
		require.Len(t, l.Last14, 14, "every row needs 14 points to draw a sparkline")
		require.NotEmpty(t, l.Created)
	}
	require.Equal(t, "active", byCode["gopher"])
	require.Equal(t, "expired", byCode["k6-run"])
}

func TestDeletingALinkStopsItResolvingImmediately(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)

	// warm the cache first, so a stale cache would keep the link alive
	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/gopher", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)

	require.Equal(t, http.StatusOK, e.adminReq(t, http.MethodDelete, "/api/admin/links/gopher").StatusCode)

	res, err = e.app.Test(httptest.NewRequest(http.MethodGet, "/gopher", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode, "deletion must not wait for the cache TTL")
}

func TestInvalidateCacheDropsTheEntryWithoutDeletingTheLink(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)
	_, _ = e.app.Test(httptest.NewRequest(http.MethodGet, "/gopher", nil))

	require.NoError(t, e.rdb.Get(t.Context(), "link:gopher").Err(), "cache should be warm")

	res := e.adminReq(t, http.MethodPost, "/api/admin/links/gopher/invalidate-cache")
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.Error(t, e.rdb.Get(t.Context(), "link:gopher").Err(), "cache entry should be gone")

	after, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/gopher", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, after.StatusCode, "the link itself must survive")
}

func TestDeletingAndInvalidatingAnUnknownCodeIsNotFound(t *testing.T) {
	e := newEnv(t)

	require.Equal(t, http.StatusNotFound, e.adminReq(t, http.MethodDelete, "/api/admin/links/nosuch").StatusCode)
	require.Equal(t, http.StatusNotFound, e.adminReq(t, http.MethodPost, "/api/admin/links/nosuch/invalidate-cache").StatusCode)
}
