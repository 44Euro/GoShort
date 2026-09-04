package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"goshort/internal/config"
)

// หน้า /metrics ของ instance ที่ถูกเฝ้า 100 ตัวอย่างตกอยู่ใน bucket 5ms ทั้งหมด
// p99 จึงอยู่ที่ 0.0025 + (0.005-0.0025)*99/100 = 0.004975 วินาที
const watchedInstanceMetrics = `# TYPE goshort_cache_lookups_total counter
goshort_cache_lookups_total{result="hit"} 90
goshort_cache_lookups_total{result="miss"} 10
# TYPE goshort_click_events_dropped_total counter
goshort_click_events_dropped_total 7
# TYPE goshort_click_events_written_total counter
goshort_click_events_written_total 1234
# TYPE goshort_click_queue_depth gauge
goshort_click_queue_depth 42
# TYPE goshort_click_queue_capacity gauge
goshort_click_queue_capacity 1000
# TYPE goshort_redirect_duration_seconds histogram
goshort_redirect_duration_seconds_bucket{le="0.0005"} 0
goshort_redirect_duration_seconds_bucket{le="0.001"} 0
goshort_redirect_duration_seconds_bucket{le="0.0025"} 0
goshort_redirect_duration_seconds_bucket{le="0.005"} 100
goshort_redirect_duration_seconds_bucket{le="+Inf"} 100
goshort_redirect_duration_seconds_sum 0.3
goshort_redirect_duration_seconds_count 100
`

type liveSummary struct {
	CacheHitRate  float64 `json:"cache_hit_rate"`
	P99Millis     float64 `json:"p99_redirect_ms"`
	QueueDepth    int64   `json:"queue_depth"`
	QueueCapacity int64   `json:"queue_capacity"`
	Dropped       int64   `json:"dropped_events"`
	Written       int64   `json:"written_events"`
}

func watching(url string) func(*config.Config) {
	return func(c *config.Config) { c.MetricsSourceURL = url }
}

// คอนโซลอยู่คนละโปรเซสกับตัวที่รับ redirect ตัวเลขวัดผลเป็นของใครของมัน ถ้าคอนโซล
// รายงานของตัวเองมันจะรายงานว่าไม่มีอะไรเกิดขึ้นเสมอ
func TestTheConsoleReportsTheMetricsOfTheInstanceItWatches(t *testing.T) {
	watched := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(watchedInstanceMetrics))
	}))
	defer watched.Close()

	e := newEnv(t, watching(watched.URL))
	cookie := e.admin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary", nil)
	req.AddCookie(cookie)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var got liveSummary
	decode(t, res, &got)
	require.InDelta(t, 90.0, got.CacheHitRate, 0.001)
	require.InDelta(t, 4.975, got.P99Millis, 0.001)
	require.Equal(t, int64(42), got.QueueDepth)
	require.Equal(t, int64(1000), got.QueueCapacity)
	require.Equal(t, int64(7), got.Dropped)
	require.Equal(t, int64(1234), got.Written)
}

// ศูนย์แปลว่า "ไม่มีอะไรเกิดขึ้น" ซึ่งคนละเรื่องกับ "มองไม่เห็น" — หน้าจอเฝ้าระบบที่
// สับสนสองอย่างนี้แย่กว่าไม่มีหน้าจอเลย
func TestTheConsoleSaysItCannotSeeRatherThanReportingZeroes(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := dead.URL
	dead.Close()

	e := newEnv(t, watching(url))
	cookie := e.admin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary", nil)
	req.AddCookie(cookie)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, res.StatusCode,
		"an unreachable source must not be reported as an idle one")
	require.NotContains(t, readBody(t, res), "cache_hit_rate",
		"no figures at all beats figures that are silently wrong")
}

func TestWithoutAWatchedInstanceTheConsoleReportsItsOwnProcess(t *testing.T) {
	e := newEnv(t)
	cookie := e.admin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary", nil)
	req.AddCookie(cookie)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var got liveSummary
	decode(t, res, &got)
	require.Equal(t, int64(1000), got.QueueCapacity, "its own pool, as before")
}

// หน้า landing ของ instance ที่เฝ้าคนอื่นอยู่ต้องรายงานตัวเลขของตัวที่มันเฝ้าเช่นกัน
// ตัวเลขของโปรเซสที่ไม่ได้รับ traffic ไม่มีความหมายกับใคร
func TestThePublicFiguresFollowTheSameSource(t *testing.T) {
	watched := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(watchedInstanceMetrics))
	}))
	defer watched.Close()

	e := newEnv(t, watching(watched.URL))

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/api/stats/public", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var got liveSummary
	decode(t, res, &got)
	require.InDelta(t, 90.0, got.CacheHitRate, 0.001)
	require.Equal(t, int64(7), got.Dropped)
}
