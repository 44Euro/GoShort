package handler_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goshort/internal/model"
)

func TestClicksAreRecordedWithoutTheRawIPAddress(t *testing.T) {
	e := newEnv(t)
	app, db := e.app, e.db
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)

	req := httptest.NewRequest(http.MethodGet, "/"+created.Code, nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("User-Agent", "curl/8.4.0")
	req.Header.Set("Referer", "https://news.ycombinator.com/")
	_, err := app.Test(req)
	require.NoError(t, err)

	var ev model.ClickEvent
	require.Eventually(t, func() bool {
		return db.Where("1 = 1").Order("id desc").First(&ev).Error == nil
	}, 3*time.Second, 20*time.Millisecond, "the click should reach postgres")

	require.Len(t, ev.IPHash, 64, "the address must be stored as a sha256 hash")
	require.NotContains(t, ev.IPHash, "203.0.113.7")
	require.Equal(t, "curl/8.4.0", ev.UserAgent)
	require.Equal(t, "https://news.ycombinator.com/", ev.Referrer)
}

// นี่คือเทสต์ที่โปรเจกต์นี้มีไว้เพื่อ: หลาย worker บวกยอดของลิงก์เดียวกันพร้อมกัน
// ต้องไม่มีคลิกไหนหายไป รันใต้ -race
func TestConcurrentClicksNeverLoseCount(t *testing.T) {
	e := newEnv(t)
	app, db := e.app, e.db
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)

	const clicks = 300
	var wg sync.WaitGroup
	for i := 0; i < clicks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/"+created.Code, nil)
			res, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusFound, res.StatusCode)
		}()
	}
	wg.Wait()

	e.drain(t)

	var link model.Link
	require.NoError(t, db.Where("short_code = ?", created.Code).First(&link).Error)
	require.Equal(t, int64(clicks), link.ClickCount)

	var rows int64
	db.Model(&model.ClickEvent{}).Where("link_id = ?", link.ID).Count(&rows)
	require.Equal(t, int64(clicks), rows)
}

func TestRedirectAnswersEvenWhenTheClickBufferIsFull(t *testing.T) {
	app := newApp(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
			require.NoError(t, err)
			require.Equal(t, http.StatusFound, res.StatusCode)
		}
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("redirects stalled; the analytics path is blocking the user")
	}
}

func TestMetricsEndpointExposesTheRedirectHistogram(t *testing.T) {
	app := newApp(t)
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)
	_, _ = app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	body := readBody(t, res)
	require.Contains(t, body, "goshort_redirect_duration_seconds_bucket")
	require.Contains(t, body, "goshort_cache_lookups_total")
	require.Contains(t, body, "goshort_click_queue_depth")
	// bucket ปริยายของ Prometheus ไม่มีขอบละเอียดระดับต่ำกว่า 5ms
	require.Contains(t, body, `le="0.0005"`)
	require.Contains(t, body, `le="0.0025"`)
}

func TestPublicStatsReadsFromTheSameRegistry(t *testing.T) {
	e := newEnv(t)
	app := e.app
	_, created := createLink(t, app, `{"long_url":"https://go.dev/"}`)
	for i := 0; i < 3; i++ {
		_, _ = app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	}
	e.drain(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/stats/public", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		CacheHitRate float64 `json:"cache_hit_rate"`
		P99          float64 `json:"p99_redirect_ms"`
		Dropped      int64   `json:"dropped_events"`
		TotalClicks  int64   `json:"total_clicks"`
	}
	decode(t, res, &body)

	// miss หนึ่งครั้งแล้ว hit อีกสอง
	require.InDelta(t, 66.6, body.CacheHitRate, 1.0)
	require.Greater(t, body.P99, 0.0)
	require.Equal(t, int64(0), body.Dropped)
	require.Equal(t, int64(3), body.TotalClicks)
}
