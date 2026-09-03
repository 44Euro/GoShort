package handler

import (
	"io/fs"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"goshort/internal/config"
	"goshort/internal/metrics"
	"goshort/internal/model"
	"goshort/internal/repository"
	"goshort/internal/worker"
)

type Deps struct {
	Assets  fs.FS
	DB      *gorm.DB
	Redis   *redis.Client
	Cfg     config.Config
	Pool    *worker.Pool
	Clicks  *repository.ClickStore
	Metrics *metrics.Metrics
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
	registerAdmin(app, d)
	registerStats(app, d)

	if d.Assets != nil {
		registerSPA(app, d.Assets)
	}

	// ต้องอยู่ท้ายสุด
	registerRedirect(app, d)

	return app
}

func (d Deps) syncPoolCounters() {
	if d.Pool == nil {
		return
	}
	d.Metrics.SetDropped(d.Pool.Dropped())
	d.Metrics.SetWritten(d.Pool.Written())
}

func (d Deps) totalClicks(c *fiber.Ctx) int64 {
	var total int64
	d.DB.WithContext(c.UserContext()).Model(&model.Link{}).
		Select("COALESCE(SUM(click_count), 0)").Scan(&total)
	return total
}
