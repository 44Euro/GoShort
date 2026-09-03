package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type dayPoint struct {
	Day    string `json:"day"`
	Clicks int64  `json:"clicks"`
}

type referrer struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

func (e env) clickFrom(t *testing.T, code, referer string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)
}

func TestPublicStatsCoverClicksSeriesAndReferrers(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)

	e.clickFrom(t, "gopher", "https://news.ycombinator.com/")
	e.clickFrom(t, "gopher", "https://news.ycombinator.com/")
	e.clickFrom(t, "gopher", "")
	e.drain(t)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/api/links/gopher/stats", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		Clicks    int64      `json:"clicks"`
		Series    []dayPoint `json:"series"`
		Referrers []referrer `json:"referrers"`
	}
	decode(t, res, &body)

	require.Equal(t, int64(3), body.Clicks)
	require.Len(t, body.Series, 14, "the chart needs a point for every day, including empty ones")
	require.Equal(t, int64(3), body.Series[13].Clicks, "today should hold all three clicks")

	names := map[string]float64{}
	for _, r := range body.Referrers {
		names[r.Name] = r.Percent
	}
	require.InDelta(t, 66.6, names["https://news.ycombinator.com/"], 1.0)
	require.InDelta(t, 33.3, names["(direct)"], 1.0)
}

func TestPublicStatsForAnUnknownCodeIsNotFound(t *testing.T) {
	app := newApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/links/nosuch/stats", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestAdminAnalyticsExposeCacheStateAndTruncatedHashes(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)
	e.clickFrom(t, "gopher", "https://github.com/")
	e.drain(t)

	res := e.adminGet(t, "/api/admin/links/gopher/analytics")
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		Clicks int64 `json:"clicks"`
		Unique int64 `json:"unique"`
		Cache  struct {
			Warm bool `json:"warm"`
			TTL  int  `json:"ttl_seconds"`
		} `json:"cache"`
		Events []struct {
			IPHash    string `json:"ip_hash"`
			Referrer  string `json:"referrer"`
			UserAgent string `json:"user_agent"`
			Time      string `json:"time"`
		} `json:"events"`
		Series []dayPoint `json:"series"`
	}
	decode(t, res, &body)

	require.Equal(t, int64(1), body.Clicks)
	require.Equal(t, int64(1), body.Unique)
	require.True(t, body.Cache.Warm, "the click above warmed the cache")
	require.Greater(t, body.Cache.TTL, 0)
	require.Len(t, body.Series, 14)

	require.Len(t, body.Events, 1)
	require.Len(t, body.Events[0].IPHash, 8, "the full hash must never leave the server")
	require.Equal(t, "https://github.com/", body.Events[0].Referrer)
}

func TestAnalyticsReportsAColdCacheAfterInvalidation(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)
	e.clickFrom(t, "gopher", "")

	e.adminReq(t, http.MethodPost, "/api/admin/links/gopher/invalidate-cache")

	res := e.adminGet(t, "/api/admin/links/gopher/analytics")
	var body struct {
		Cache struct {
			Warm bool `json:"warm"`
		} `json:"cache"`
	}
	decode(t, res, &body)
	require.False(t, body.Cache.Warm)
}

func TestDashboardSummaryCarriesTheLiveGauges(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)
	e.clickFrom(t, "gopher", "")
	e.clickFrom(t, "gopher", "")
	e.drain(t)

	res := e.adminGet(t, "/api/admin/dashboard/summary")
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		CacheHitRate float64 `json:"cache_hit_rate"`
		P99          float64 `json:"p99_redirect_ms"`
		QueueDepth   int64   `json:"queue_depth"`
		QueueCap     int64   `json:"queue_capacity"`
		Written      int64   `json:"written_events"`
		TotalClicks  int64   `json:"total_clicks"`
	}
	decode(t, res, &body)

	require.Equal(t, int64(1000), body.QueueCap)
	require.Equal(t, int64(0), body.QueueDepth)
	require.Equal(t, int64(2), body.Written)
	require.Equal(t, int64(2), body.TotalClicks)
	require.Greater(t, body.P99, 0.0)
	require.InDelta(t, 50.0, body.CacheHitRate, 1.0)
}

func TestDashboardOverviewCarriesTheHeavyAggregates(t *testing.T) {
	e := newEnv(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/","alias":"gopher"}`)
	createLink(t, e.app, `{"long_url":"https://k6.io/","alias":"k6-run"}`)
	e.clickFrom(t, "gopher", "https://github.com/")
	e.clickFrom(t, "gopher", "https://github.com/")
	e.clickFrom(t, "k6-run", "")
	e.drain(t)

	res := e.adminGet(t, "/api/admin/dashboard/overview")
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		Series   []dayPoint `json:"series"`
		TopLinks []struct {
			Rank   int    `json:"rank"`
			Code   string `json:"code"`
			Clicks int64  `json:"clicks"`
		} `json:"top_links"`
		Referrers []referrer `json:"referrers"`
	}
	decode(t, res, &body)

	require.Len(t, body.Series, 14)
	require.Equal(t, int64(3), body.Series[13].Clicks)
	require.Len(t, body.TopLinks, 2)
	require.Equal(t, "gopher", body.TopLinks[0].Code)
	require.Equal(t, int64(2), body.TopLinks[0].Clicks)
	require.NotEmpty(t, body.Referrers)
}
