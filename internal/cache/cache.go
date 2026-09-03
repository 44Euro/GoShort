package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type LinkCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewLinkCache(rdb *redis.Client, ttl time.Duration) *LinkCache {
	return &LinkCache{rdb: rdb, ttl: ttl}
}

func key(code string) string { return "link:" + code }

// cache ล่มไม่ใช่เหตุให้ผู้ใช้เห็น error — คืน miss แล้วให้ผู้เรียกไปต่อที่ Postgres
func (c *LinkCache) Get(ctx context.Context, code string) (string, bool) {
	v, err := c.rdb.Get(ctx, key(code)).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (c *LinkCache) Set(ctx context.Context, code, longURL string) {
	c.rdb.Set(ctx, key(code), longURL, c.ttl)
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
