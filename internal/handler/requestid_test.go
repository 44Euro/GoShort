package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"goshort/internal/middleware"
)

func TestEveryResponseCarriesARequestId(t *testing.T) {
	app := newApp(t)

	for _, path := range []string{"/health", "/api/stats/public", "/nosuchcode"} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		require.NotEmpty(t, res.Header.Get(middleware.RequestIDHeader),
			"%s must be traceable in the log", path)
	}
}

// ตัวระบุที่ proxy ต้นทางตั้งไว้ต้องรอดมาถึงที่นี่ ไม่งั้นเรื่องเดียวกันจะขาดเป็นสองท่อน
func TestAnIncomingRequestIdIsKeptNotReplaced(t *testing.T) {
	app := newApp(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(middleware.RequestIDHeader, "from-the-edge-42")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "from-the-edge-42", res.Header.Get(middleware.RequestIDHeader))
}

// ค่าจากภายนอกไหลลง log ตรง ๆ ปล่อยให้ยาวเท่าไหร่ก็ได้คือเปิดให้ใครก็ถม log ได้
// (fasthttp ปัด header ที่ใหญ่เกิน read buffer ทิ้งตั้งแต่ชั้น transport อยู่แล้ว
// แต่ยังเหลือช่วงที่ผ่านเข้ามาได้สบาย ๆ จึงต้องมี cap ของเราเองอีกชั้น)
func TestAnOverlongRequestIdIsReplaced(t *testing.T) {
	app := newApp(t)

	huge := strings.Repeat("a", 500)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(middleware.RequestIDHeader, huge)
	res, err := app.Test(req)
	require.NoError(t, err)

	got := res.Header.Get(middleware.RequestIDHeader)
	require.NotEqual(t, huge, got)
	require.NotEmpty(t, got)
	require.LessOrEqual(t, len(got), 64)
}
