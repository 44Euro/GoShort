package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// เก็บ ID มาด้วย ไม่ใช่แค่ URL — cache hit ต้องบันทึกคลิกได้โดยไม่ต้องกลับไปถาม
// Postgres เพื่อหา link_id ไม่งั้น cache ก็ไม่ได้ช่วยอะไรบน path ที่มี analytics
type Entry struct {
	ID        uint       `json:"id"`
	LongURL   string     `json:"url"`
	ExpiresAt *time.Time `json:"exp,omitempty"`
}

func (e Entry) Expired(now time.Time) bool {
	return e.ExpiresAt != nil && e.ExpiresAt.Before(now)
}

type LinkCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewLinkCache(rdb *redis.Client, ttl time.Duration) *LinkCache {
	return &LinkCache{rdb: rdb, ttl: ttl}
}

func key(code string) string { return "link:" + code }

// cache ล่มไม่ใช่เหตุให้ผู้ใช้เห็น error — คืน miss แล้วให้ผู้เรียกไปต่อที่ Postgres
func (c *LinkCache) Get(ctx context.Context, code string) (Entry, bool) {
	raw, err := c.rdb.Get(ctx, key(code)).Bytes()
	if err != nil {
		return Entry{}, false
	}
	var e Entry
	if json.Unmarshal(raw, &e) != nil {
		return Entry{}, false
	}
	return e, true
}

// TTL ของ key ต้องไม่ยาวกว่าอายุของลิงก์เอง ไม่งั้นลิงก์ที่หมดอายุแล้วจะยัง
// เด้งต่อจนกว่า cache จะหมดอายุตาม อีกชั้นคือ Entry.Expired() กันเวลาเครื่องเพี้ยน
func (c *LinkCache) Set(ctx context.Context, code string, e Entry) {
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}

	ttl := c.ttl
	if e.ExpiresAt != nil {
		remaining := time.Until(*e.ExpiresAt)
		if remaining <= 0 {
			return
		}
		if remaining < ttl {
			ttl = remaining
		}
	}
	c.rdb.Set(ctx, key(code), raw, ttl)
}

func (c *LinkCache) Invalidate(ctx context.Context, code string) error {
	return c.rdb.Del(ctx, key(code)).Err()
}

func (c *LinkCache) TTL(ctx context.Context, code string) (time.Duration, bool) {
	d, err := c.rdb.TTL(ctx, key(code)).Result()
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}
