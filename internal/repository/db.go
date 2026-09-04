package repository

import (
	"fmt"
	"log/slog"
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
	gcfg := &gorm.Config{TranslateError: true, Logger: gormLogger()}
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

// GORM พิมพ์ของตัวเองเป็นข้อความหลายบรรทัดพร้อมรหัสสี ซึ่งทำให้ log stream ที่เหลือ
// ทั้งหมดกลายเป็นของที่ parse ไม่ได้ ส่งผ่าน slog แทนเพื่อให้ทุกบรรทัดหน้าตาเดียวกัน
// และ ErrRecordNotFound ต้องไม่ถูกนับเป็น error — short code ที่ไม่มีจริงคือ 404
// ตามปกติ ไม่ใช่เหตุผิดปกติที่ต้องมีใครไปดู
func gormLogger() logger.Interface {
	return logger.New(slogWriter{}, logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
}

type slogWriter struct{}

func (slogWriter) Printf(format string, args ...any) {
	slog.Warn("gorm", "detail", fmt.Sprintf(format, args...))
}
