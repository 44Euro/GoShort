package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthReportsEveryDependency(t *testing.T) {
	app := newApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var body struct {
		Status   string `json:"status"`
		Postgres string `json:"postgres"`
		Redis    string `json:"redis"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	require.Equal(t, "ok", body.Status)
	require.Equal(t, "up", body.Postgres)
	require.Equal(t, "up", body.Redis)
}

func TestHealthIsNotSwallowedByTheRedirectRoute(t *testing.T) {
	app := newApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Empty(t, res.Header.Get("Location"))
}
