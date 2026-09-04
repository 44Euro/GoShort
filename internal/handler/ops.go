package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"goshort/internal/middleware"
)

func registerOps(app *fiber.App, d Deps) {
	// liveness ตอบจากตัวโปรเซสเท่านั้น ห้ามแตะ dependency: Postgres ที่ค้างอยู่
	// (ไม่ใช่ล่ม) จะลาก probe ไปจน timeout แล้วโปรเซสที่ยังดีอยู่โดนรีสตาร์ต
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "alive"})
	})

	app.Get("/health/ready", func(c *fiber.Ctx) error {
		pg, rds := "up", "up"

		sql, err := d.DB.DB()
		if err != nil || sql.PingContext(c.UserContext()) != nil {
			pg = "down"
		}
		if d.Redis.Ping(c.UserContext()).Err() != nil {
			rds = "down"
		}

		// Redis ล่มต้องไม่ทำให้ instance นี้หลุด rotation — redirect fallback ไป
		// Postgres ได้ และ rate limiter fail-open อยู่แล้ว มันยังตอบถูก แค่ช้าลง
		// ถอดออกตอน cache กระตุกคือการล้าง pool ทิ้งทั้งชุดพร้อมกัน
		status, code := "ready", fiber.StatusOK
		if rds == "down" {
			status = "degraded"
		}
		// ต่างจาก Redis ตรงที่ cache miss ไม่มีอะไรให้ตกลงไปต่ำกว่านี้
		if pg == "down" {
			status, code = "unready", fiber.StatusServiceUnavailable
		}

		return c.Status(code).JSON(fiber.Map{"status": status, "postgres": pg, "redis": rds})
	})

	app.Get("/metrics", adaptor.HTTPHandler(
		promhttp.HandlerFor(d.Metrics.Registry(), promhttp.HandlerOpts{}),
	))

	app.Get("/api/stats/public",
		middleware.RateLimit(d.Redis, "public-stats", 120, time.Minute),
		func(c *fiber.Ctx) error {
			s := d.Metrics.Summary()
			total, err := d.totalClicks(c)
			if err != nil {
				return err
			}
			return c.JSON(fiber.Map{
				"cache_hit_rate":  s.CacheHitRate,
				"p99_redirect_ms": s.P99Millis,
				"dropped_events":  s.Dropped,
				"total_clicks":    total,
			})
		})
}
