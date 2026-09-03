package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomAliasIsHonoured(t *testing.T) {
	app := newApp(t)

	res, body := createLink(t, app, `{"long_url":"https://go.dev/","alias":"gopher"}`)

	require.Equal(t, http.StatusCreated, res.StatusCode)
	require.Equal(t, "gopher", body.Code)
	require.Equal(t, "http://localhost:8080/gopher", body.ShortURL)
}

func TestTakenAliasIsRejected(t *testing.T) {
	app := newApp(t)
	createLink(t, app, `{"long_url":"https://go.dev/","alias":"gopher"}`)

	res, _ := createLink(t, app, `{"long_url":"https://example.com/","alias":"gopher"}`)

	require.Equal(t, http.StatusConflict, res.StatusCode)
}

func TestReservedAliasesCannotShadowApplicationRoutes(t *testing.T) {
	app := newApp(t)

	for _, alias := range []string{"api", "admin", "health", "metrics", "login", "s", "assets", "_next"} {
		res, _ := createLink(t, app, `{"long_url":"https://go.dev/","alias":"`+alias+`"}`)
		require.Equal(t, http.StatusConflict, res.StatusCode, "alias %q must be reserved", alias)
	}
}

func TestMalformedURLsAreRejected(t *testing.T) {
	app := newApp(t)

	for _, url := range []string{"", "go.dev", "ftp://go.dev", "javascript:alert(1)", "://nope"} {
		res, _ := createLink(t, app, `{"long_url":"`+url+`"}`)
		require.Equal(t, http.StatusBadRequest, res.StatusCode, "url %q must be rejected", url)
	}
}

// ย่อลิงก์ที่ชี้กลับมาที่ตัวเองได้ = สร้าง redirect loop ให้คนอื่นกดเล่น
func TestLinksPointingBackAtGoShortAreRejected(t *testing.T) {
	app := newApp(t)

	res, _ := createLink(t, app, `{"long_url":"http://localhost:8080/gopher"}`)

	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestAliasShapeIsValidated(t *testing.T) {
	app := newApp(t)

	for _, alias := range []string{"has space", "has/slash", "UPPER!", "waytoolongalias"} {
		res, _ := createLink(t, app, `{"long_url":"https://go.dev/","alias":"`+alias+`"}`)
		require.Equal(t, http.StatusBadRequest, res.StatusCode, "alias %q must be rejected", alias)
	}
}
