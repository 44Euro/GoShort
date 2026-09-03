package repository

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PoolConfig struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
	Quiet       bool
}

// ไม่จำกัด MaxOpenConns คือปล่อยให้ทราฟฟิกพุ่งเปิด connection จนชน max_connections
// ของ Postgres แล้ว redirect จะตอบ 500 แทนที่จะรอคิว — เจอตอนยิง 300 request พร้อมกัน
func Open(dsn string, cfg PoolConfig) (*gorm.DB, error) {
	gcfg := &gorm.Config{TranslateError: true}
	if cfg.Quiet {
		gcfg.Logger = logger.Discard
	}

	db, err := gorm.Open(postgres.Open(dsn), gcfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.MaxIdleTime)

	return db, nil
}
