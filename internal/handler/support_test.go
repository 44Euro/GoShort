package handler_test

import (
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"goshort/internal/config"
	"goshort/internal/handler"
	"goshort/internal/model"
)

// integration test ต้องมี postgres จริง ถ้าไม่ตั้ง env ก็ข้ามไปแทนที่จะ fail
// (CI ตั้งให้ผ่าน service container, local ตั้งเองผ่าน docker-compose)
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard, TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, model.Migrate(db))
	require.NoError(t, model.Truncate(db))
	return db
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
		BaseURL:    "http://localhost:8080",
		JWTSecret:  "test-secret",
		CacheTTL:   time.Hour,
		TrustProxy: true,
		Dev:        true,
	}
}

func newApp(t *testing.T) *fiber.App {
	t.Helper()
	app, _, _ := newAppWithDeps(t)
	return app
}

func newAppWithDeps(t *testing.T) (*fiber.App, *gorm.DB, *redis.Client) {
	t.Helper()
	db, rdb := testDB(t), testRedis(t)
	return handler.New(handler.Deps{DB: db, Redis: rdb, Cfg: testConfig()}), db, rdb
}
