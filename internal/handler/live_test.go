package handler_test

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goshort/internal/model"
)

// app.Test() สร้าง context ใหม่ทุกครั้ง จึงไม่เจอปัญหา buffer ที่ fasthttp เอากลับ
// มาใช้ซ้ำ ต้องยิงผ่าน socket จริงเท่านั้นถึงจะเห็น
func (e env) listen(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() { _ = e.app.Listener(ln) }()
	t.Cleanup(func() { _ = e.app.Shutdown() })

	base := "http://" + ln.Addr().String()
	require.Eventually(t, func() bool {
		res, err := http.Get(base + "/health")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond)
	return base
}

func TestHeadersSurviveBufferReuseOnALiveListener(t *testing.T) {
	e := newEnv(t)
	_, created := createLink(t, e.app, `{"long_url":"https://go.dev/"}`)
	base := e.listen(t)

	referrers := []string{
		"https://news.ycombinator.com/",
		"https://github.com/",
		"https://reddit.com/r/golang",
		"https://lobste.rs/",
	}
	agents := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
		"curl/8.4.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_2)",
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodGet, base+"/"+created.Code, nil)
			require.NoError(t, err)
			req.Header.Set("Referer", referrers[i%len(referrers)])
			req.Header.Set("User-Agent", agents[i%len(agents)])
			res, err := client.Do(req)
			require.NoError(t, err)
			_ = res.Body.Close()
		}(i)
	}
	wg.Wait()
	e.drain(t)

	valid := map[string]bool{}
	for _, r := range referrers {
		valid[r] = true
	}
	validUA := map[string]bool{}
	for _, a := range agents {
		validUA[a] = true
	}

	var events []model.ClickEvent
	require.NoError(t, e.db.Find(&events).Error)
	require.NotEmpty(t, events)

	for _, ev := range events {
		require.True(t, valid[ev.Referrer],
			fmt.Sprintf("referrer %q was corrupted by buffer reuse", ev.Referrer))
		require.True(t, validUA[ev.UserAgent],
			fmt.Sprintf("user agent %q was corrupted by buffer reuse", ev.UserAgent))
	}
}
