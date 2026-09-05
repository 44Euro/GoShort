package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type registerRow struct {
	Code   string  `json:"code"`
	Clicks int64   `json:"clicks"`
	Spark  []int64 `json:"last_14_days"`
}

func register(t *testing.T, e env, cookie *http.Cookie) []registerRow {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/links", nil)
	req.AddCookie(cookie)
	res, err := e.app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		Links []registerRow `json:"links"`
	}
	decode(t, res, &body)
	return body.Links
}

// หน้าทะเบียน poll ซ้ำ ๆ ได้ก็ต่อเมื่อยอดคลิกไม่ถูก cache — ยอดมาจากคอลัมน์ที่
// denormalize ไว้แล้ว อ่านถูกกว่า GROUP BY ของ sparkline หลายร้อยเท่า
func TestTheRegisterReportsClickCountsWithoutCachingThem(t *testing.T) {
	e := newEnv(t)
	cookie := e.admin(t)
	_, created := createLink(t, e.app, `{"long_url":"https://go.dev/"}`)

	_, err := e.app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	e.drain(t)

	rows := register(t, e, cookie)
	require.Len(t, rows, 1)
	require.Equal(t, int64(1), rows[0].Clicks)

	_, err = e.app.Test(httptest.NewRequest(http.MethodGet, "/"+created.Code, nil))
	require.NoError(t, err)
	e.drain(t)

	rows = register(t, e, cookie)
	require.Equal(t, int64(2), rows[0].Clicks,
		"a cached count would make the register lie every time someone clicks")
}

// ส่วนที่แพงคือ GROUP BY 14 วัน ไม่ใช่ยอดคลิก จึง cache ได้โดยไม่มีใครสังเกตเห็น
func TestTheRegisterCachesTheExpensiveSeriesOnly(t *testing.T) {
	e := newEnv(t)
	cookie := e.admin(t)
	createLink(t, e.app, `{"long_url":"https://go.dev/"}`)

	rows := register(t, e, cookie)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Spark, 14, "the sparkline still has to be there")

	keys, err := e.rdb.Keys(t.Context(), "*series*").Result()
	require.NoError(t, err)
	require.NotEmpty(t, keys, "the fourteen-day rollup should be cached, not rebuilt on every poll")
}
