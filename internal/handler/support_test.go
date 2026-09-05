package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"goshort/internal/config"
	"goshort/internal/handler"
	"goshort/internal/metrics"
	"goshort/internal/model"
	"goshort/internal/repository"
	"goshort/internal/worker"
	"goshort/web"
)

// เทสต์อยู่ในสคีมาของตัวเอง ไม่ใช่ public — TEST_DATABASE_URL ชี้ไปที่ database
// เดียวกับที่แอปรันอยู่ได้โดยไม่ล้างลิงก์ที่กำลังดูอยู่ทิ้ง ห้ามใส่ public ต่อท้าย
// search_path เด็ดขาด ไม่งั้น AutoMigrate จะเจอตารางใน public แล้วไม่สร้างของตัวเอง
// ทำให้เทสต์กลับไปเขียนทับข้อมูลจริงเงียบ ๆ
const testSchema = "goshort_test"

var schemaOnce sync.Once

// integration test ต้องมี postgres จริง ถ้าไม่ตั้ง env ก็ข้ามไปแทนที่จะ fail
// (CI ตั้งให้ผ่าน service container, local ตั้งเองผ่าน docker-compose)
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ensureTestSchema(t, dsn)

	db, err := repository.Open(scopedToTestSchema(dsn), repository.PoolConfig{
		MaxOpen: 10, MaxIdle: 10, MaxLifetime: time.Minute, MaxIdleTime: time.Minute, Quiet: true,
	})
	require.NoError(t, err)

	// ทุกเทสต์เปิด pool ของตัวเอง ถ้าไม่ปิดจะสะสมจนชน max_connections ของ Postgres
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, model.Migrate(db))
	require.NoError(t, model.Truncate(db))
	return db
}

// สคีมาต้องมีอยู่ก่อนถึงจะต่อเข้าไปด้วย search_path ได้ ทำครั้งเดียวต่อการรันหนึ่งชุด
func ensureTestSchema(t *testing.T, dsn string) {
	t.Helper()
	var err error
	schemaOnce.Do(func() {
		var db *gorm.DB
		db, err = repository.Open(dsn, repository.PoolConfig{
			MaxOpen: 1, MaxIdle: 1, MaxLifetime: time.Minute, MaxIdleTime: time.Minute, Quiet: true,
		})
		if err != nil {
			return
		}
		defer func() {
			if sqlDB, e := db.DB(); e == nil {
				_ = sqlDB.Close()
			}
		}()
		err = db.Exec("CREATE SCHEMA IF NOT EXISTS " + testSchema).Error
	})
	require.NoError(t, err)
}

func scopedToTestSchema(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + testSchema
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func testConfig() config.Config {
	return config.Config{
		BaseURL:         "http://localhost:8080",
		JWTSecret:       "test-secret",
		CacheTTL:        time.Hour,
		TrustProxy:      true,
		AdminEnabled:    true,
		ClickBufferSize: 1000,
		ClickBatchSize:  50,
	}
}

type env struct {
	app  *fiber.App
	db   *gorm.DB
	rdb  *redis.Client
	pool *worker.Pool
}

func newApp(t *testing.T) *fiber.App {
	t.Helper()
	return newEnv(t).app
}

func newAppWithDeps(t *testing.T) (*fiber.App, *gorm.DB, *redis.Client) {
	t.Helper()
	e := newEnv(t)
	return e.app, e.db, e.rdb
}

func newEnvWithAssets(t *testing.T, tweaks ...func(*config.Config)) env {
	t.Helper()
	return build(t, web.Dist(), tweaks...)
}

func newEnv(t *testing.T, tweaks ...func(*config.Config)) env {
	t.Helper()
	return build(t, nil, tweaks...)
}

func build(t *testing.T, assets fs.FS, tweaks ...func(*config.Config)) env {
	t.Helper()
	db, rdb := testDB(t), testRedis(t)
	cfg := testConfig()
	for _, tweak := range tweaks {
		tweak(&cfg)
	}

	clicks := repository.NewClickStore(db)
	pool := worker.New(clicks, worker.Config{
		Workers:    2,
		Buffer:     cfg.ClickBufferSize,
		BatchSize:  cfg.ClickBatchSize,
		FlushEvery: 20 * time.Millisecond,
	})
	pool.Start()
	t.Cleanup(func() { _ = pool.Shutdown(context.Background()) })

	m := metrics.New()
	m.TrackPool(pool)

	app := handler.New(handler.Deps{
		Assets: assets,
		DB:     db, Redis: rdb, Cfg: cfg,
		Pool: pool, Clicks: clicks, Metrics: m,
	})
	return env{app: app, db: db, rdb: rdb, pool: pool}
}

func (e env) seedAdmin(t *testing.T, email, password string) {
	t.Helper()
	require.NoError(t, repository.NewAdminRepo(e.db).Upsert(context.Background(), email, password))
}

// รอให้ pipeline เขียนของค้างจนหมด: คิวว่าง แล้วยอดที่เขียนไปนิ่งข้ามรอบ flush
func (e env) drain(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		if e.pool.Depth() != 0 {
			return false
		}
		before := e.pool.Written()
		time.Sleep(60 * time.Millisecond)
		return e.pool.Depth() == 0 && e.pool.Written() == before
	}, 10*time.Second, 20*time.Millisecond)
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return string(b)
}

func decode(t *testing.T, res *http.Response, v any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(res.Body).Decode(v))
}

func (e env) admin(t *testing.T) *http.Cookie {
	t.Helper()
	e.seedAdmin(t, "admin@goshort.dev", "goshort-demo")
	return sessionCookie(t, login(t, e.app, "admin@goshort.dev", "goshort-demo"))
}
