package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func registerOps(app *fiber.App, d Deps) {
	app.Get("/health", func(c *fiber.Ctx) error {
		pg, rds := "up", "up"

		sql, err := d.DB.DB()
		if err != nil || sql.PingContext(c.Context()) != nil {
			pg = "down"
		}
		if d.Redis.Ping(c.Context()).Err() != nil {
			rds = "down"
		}

		status := "ok"
		code := fiber.StatusOK
		if pg == "down" || rds == "down" {
			status, code = "degraded", fiber.StatusServiceUnavailable
		}

		return c.Status(code).JSON(fiber.Map{"status": status, "postgres": pg, "redis": rds})
	})

	app.Get("/metrics", adaptor.HTTPHandler(
		promhttp.HandlerFor(d.Metrics.Registry(), promhttp.HandlerOpts{}),
	))

	app.Get("/api/stats/public", func(c *fiber.Ctx) error {
		d.syncPoolCounters()
		s := d.Metrics.Summary()
		return c.JSON(fiber.Map{
			"cache_hit_rate":  s.CacheHitRate,
			"p99_redirect_ms": s.P99Millis,
			"dropped_events":  s.Dropped,
			"total_clicks":    d.totalClicks(c),
		})
	})
}
