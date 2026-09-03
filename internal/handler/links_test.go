package handler_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type createResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
	LongURL  string `json:"long_url"`
}

func createLink(t *testing.T, app *fiber.App, body string) (*http.Response, createResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, -1)
	require.NoError(t, err)

	var out createResponse
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	return res, out
}

func TestCreateLinkReturnsAShortCodeAndFullURL(t *testing.T) {
	app := newApp(t)

	res, body := createLink(t, app, `{"long_url":"https://go.dev/blog/pipelines"}`)

	require.Equal(t, http.StatusCreated, res.StatusCode)
	require.Len(t, body.Code, 7)
	require.Equal(t, "https://go.dev/blog/pipelines", body.LongURL)
	require.Equal(t, "http://localhost:8080/"+body.Code, body.ShortURL)
}

func TestGeneratedCodesAvoidVisuallyConfusableCharacters(t *testing.T) {
	app := newApp(t)

	for i := 0; i < 40; i++ {
		_, body := createLink(t, app, `{"long_url":"https://go.dev/"}`)
		require.NotContains(t, body.Code, "0")
		require.NotContains(t, body.Code, "O")
		require.NotContains(t, body.Code, "1")
		require.NotContains(t, body.Code, "l")
		require.NotContains(t, body.Code, "I")
	}
}

func TestConcurrentCreatesNeverHandOutTheSameCode(t *testing.T) {
	app := newApp(t)

	const n = 60
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, body := createLink(t, app, `{"long_url":"https://go.dev/"}`)
			if res.StatusCode != http.StatusCreated {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			seen[body.Code] = true
		}()
	}
	wg.Wait()

	require.Len(t, seen, n, "every concurrent create should get a distinct code")
}

func TestCreateLinkAcceptsAnExpiry(t *testing.T) {
	app := newApp(t)

	res, body := createLink(t, app, `{"long_url":"https://go.dev/","expires_at":"2030-01-01T00:00:00Z"}`)

	require.Equal(t, http.StatusCreated, res.StatusCode)
	require.NotEmpty(t, body.Code)
}
