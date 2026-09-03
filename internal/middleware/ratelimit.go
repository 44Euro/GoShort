package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// INCR แล้วตั้ง EXPIRE เฉพาะครั้งแรกของหน้าต่าง ทำให้หน้าต่างเลื่อนเป็นช่วง ๆ
// ไม่ใช่ sliding window จริง แต่พอสำหรับกันการยิงรัวและอ่านง่ายกว่ามาก
func RateLimit(rdb *redis.Client, name string, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := "rl:" + name + ":" + c.IP()
		ctx := c.UserContext()

		n, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis ล่มแล้วปิดประตูใส่ผู้ใช้ทั้งหมดแย่กว่าปล่อยผ่านชั่วคราว
			return c.Next()
		}
		if n == 1 {
			rdb.Expire(ctx, key, window)
		}

		if n > int64(limit) {
			retry := window
			if ttl, err := rdb.TTL(ctx, key).Result(); err == nil && ttl > 0 {
				retry = ttl
			}
			c.Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			return c.Status(fiber.StatusTooManyRequests).
				JSON(fiber.Map{"error": "too many requests, slow down"})
		}

		c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-int(n))))
		return c.Next()
	}
}
