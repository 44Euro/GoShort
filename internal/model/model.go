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

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Link{}, &ClickEvent{}, &AdminUser{})
}

func Truncate(db *gorm.DB) error {
	return db.Exec("TRUNCATE click_events, links, admin_users RESTART IDENTITY CASCADE").Error
}
