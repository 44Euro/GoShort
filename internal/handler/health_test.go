package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type readiness struct {
	Status   string `json:"status"`
	Postgres string `json:"postgres"`
	Redis    string `json:"redis"`
}

// liveness ต้องตอบได้โดยไม่แตะอะไรข้างนอกเลย ไม่งั้น Postgres ที่ค้าง (ไม่ใช่ล่ม)
// จะทำให้ probe timeout แล้ว orchestrator รีสตาร์ตโปรเซสที่ไม่ได้พัง
func TestLivenessStaysUpWhileEveryDependencyIsDown(t *testing.T) {
	e := newEnv(t)

	require.NoError(t, e.rdb.Close())
	sqlDB, err := e.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode,
		"liveness answers for the process, not for what the process talks to")
}

func TestReadinessReportsEveryDependency(t *testing.T) {
	e := newEnv(t)

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body readiness
	decode(t, res, &body)
	require.Equal(t, "ready", body.Status)
	require.Equal(t, "up", body.Postgres)
	require.Equal(t, "up", body.Redis)
}

// แก่นของ ticket นี้: cache ล่มแล้ว redirect ยัง fallback ไป Postgres ได้และ
// rate limiter ก็ fail-open อยู่แล้ว instance นี้ยังเสิร์ฟถูกต้อง แค่ช้าลง
// ถอดมันออกจาก rotation ตอน Redis กระตุกคือการล้าง pool ทิ้งทั้งชุดพร้อมกัน
func TestReadinessStaysGreenWhenOnlyTheCacheIsDown(t *testing.T) {
	e := newEnv(t)
	require.NoError(t, e.rdb.Close())

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode, "a dead cache must not empty the load balancer")

	var body readiness
	decode(t, res, &body)
	require.Equal(t, "degraded", body.Status)
	require.Equal(t, "up", body.Postgres)
	require.Equal(t, "down", body.Redis)
}

func TestReadinessGoesRedWhenPostgresIsUnreachable(t *testing.T) {
	e := newEnv(t)
	sqlDB, err := e.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	res, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, res.StatusCode,
		"a cache miss has nothing to fall back to without postgres")

	var body readiness
	decode(t, res, &body)
	require.Equal(t, "unready", body.Status)
	require.Equal(t, "down", body.Postgres)
}

func TestHealthChecksAreNotSwallowedByTheRedirectRoute(t *testing.T) {
	app := newApp(t)

	for _, path := range []string{"/health", "/health/ready"} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Empty(t, res.Header.Get("Location"), "%s must never redirect", path)
	}
}
