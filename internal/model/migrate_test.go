package model_test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"goshort/internal/model"
	"goshort/internal/repository"
)

// AutoMigrate เช็คว่า index มีอยู่แล้วหรือยังก่อนสร้าง ซึ่งเป็น TOCTOU ล้วน ๆ
// สอง instance ที่ boot พร้อมกันบน schema ใหม่จะผ่าน check พร้อมกันแล้วชนกันตอน CREATE
func TestConcurrentMigrationsDoNotCollide(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	schema := fmt.Sprintf("migrate_race_%d", time.Now().UnixNano())
	admin := open(t, dsn)
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}

	const instances = 4
	dbs := make([]*gorm.DB, instances)
	for i := range dbs {
		dbs[i] = open(t, dsn+sep+"search_path="+schema)
	}

	var ready sync.WaitGroup
	ready.Add(1)
	errs := make(chan error, instances)
	for _, db := range dbs {
		go func() {
			ready.Wait()
			errs <- model.Migrate(db)
		}()
	}
	ready.Done()

	for range instances {
		require.NoError(t, <-errs, "instances booting together must not fight over the schema")
	}
}

func open(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := repository.Open(dsn, repository.PoolConfig{
		MaxOpen: 4, MaxIdle: 4, MaxLifetime: time.Minute, MaxIdleTime: time.Minute, Quiet: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
