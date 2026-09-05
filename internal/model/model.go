package model

import (
	"time"

	"gorm.io/gorm"
)

type Link struct {
	ID          uint   `gorm:"primaryKey"`
	ShortCode   string `gorm:"uniqueIndex;size:12;not null"`
	LongURL     string `gorm:"not null"`
	CustomAlias bool   `gorm:"default:false"`
	ExpiresAt   *time.Time
	ClickCount  int64 `gorm:"default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ClickEvent struct {
	ID        uint   `gorm:"primaryKey"`
	LinkID    uint   `gorm:"not null;index:idx_click_link_time,priority:1"`
	IPHash    string `gorm:"size:64"`
	UserAgent string
	Referrer  string
	CreatedAt time.Time `gorm:"index:idx_click_link_time,priority:2"`
}

type AdminUser struct {
	ID           uint   `gorm:"primaryKey"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

const migrationLock = 4021873

// AutoMigrate ถาม Postgres ว่ามี index อยู่แล้วหรือยังก่อนสั่งสร้าง สอง instance
// ที่ boot พร้อมกันบน schema ใหม่จึงผ่าน check พร้อมกันแล้วชนกันตอน CREATE
// ล็อกระดับ transaction ให้ผ่านทีละตัว แล้วปล่อยเองตอน commit — ใช้ pg_advisory_lock
// ธรรมดาไม่ได้เพราะ lock ผูกกับ connection ส่วน unlock อาจไปโดนอีก connection ใน pool
func Migrate(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", migrationLock).Error; err != nil {
			return err
		}
		return tx.AutoMigrate(&Link{}, &ClickEvent{}, &AdminUser{})
	})
}

func Truncate(db *gorm.DB) error {
	// ไม่ใส่ RESTART IDENTITY: ถ้ารีเซ็ต sequence แล้ว id จะถูกใช้ซ้ำ ซึ่งอันตราย
	// เมื่อ test ชี้มาที่ database เดียวกับแอปที่กำลังรันอยู่ — entry ที่ค้างใน Redis
	// ยังถือ id เดิม แล้วคลิกจะไปลงผิดลิงก์เงียบ ๆ ไม่มี test ไหนพึ่ง id ที่เจาะจง
	return db.Exec("TRUNCATE click_events, links, admin_users CASCADE").Error
}
