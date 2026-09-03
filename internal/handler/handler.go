package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"goshort/internal/config"
)

type Deps struct {
	DB    *gorm.DB
	Redis *redis.Client
	Cfg   config.Config
}

// ลำดับการลงทะเบียนสำคัญ: Fiber match ตามลำดับ และ GET /:code เป็น wildcard
// ที่ root มันจะกินทุก path ที่ลงทะเบียนทีหลัง จึงต้องอยู่ท้ายสุดเสมอ
func New(d Deps) *fiber.App {
	fc := fiber.Config{
		AppName:               "goshort",
		DisableStartupMessage: true,
	}
	if d.Cfg.TrustProxy {
		fc.ProxyHeader = fiber.HeaderXForwardedFor
	}
	app := fiber.New(fc)

	registerOps(app, d)
	registerLinks(app, d)

	// ต้องอยู่ท้ายสุด
	registerRedirect(app, d)

	return app
}

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

		return c.Status(code).JSON(fiber.Map{
			"status":   status,
			"postgres": pg,
			"redis":    rds,
		})
	})
}
