package handler

import (
	"io/fs"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"goshort/internal/cache"
	"goshort/internal/config"
	"goshort/internal/metrics"
	"goshort/internal/middleware"
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

	links  *repository.LinkRepo
	stats  *repository.StatsRepo
	admins *repository.AdminRepo
	cache  *cache.LinkCache
	index  []byte
}

// ลำดับการลงทะเบียนสำคัญ: Fiber match ตามลำดับ และ GET /:code เป็น wildcard
// ที่ root มันจะกินทุก path ที่ลงทะเบียนทีหลัง จึงต้องอยู่ท้ายสุดเสมอ
func New(d Deps) *fiber.App {
	d.links = repository.NewLinkRepo(d.DB)
	d.stats = repository.NewStatsRepo(d.DB)
	d.admins = repository.NewAdminRepo(d.DB)
	d.cache = cache.NewLinkCache(d.Redis, d.Cfg.CacheTTL)
	if d.Assets != nil {
		d.index = spaShell(d.Assets, d.Cfg.AdminEnabled)
	}

	fc := fiber.Config{
		AppName:               "goshort",
		DisableStartupMessage: true,
	}
	if d.Cfg.TrustProxy {
		fc.ProxyHeader = fiber.HeaderXForwardedFor
	}
	app := fiber.New(fc)

	app.Use(middleware.RequestID(), middleware.AccessLog())

	registerOps(app, d)
	registerLinks(app, d)
	if d.Cfg.AdminEnabled {
		registerAdmin(app, d)
	}
	registerStats(app, d)

	if d.Assets != nil {
		registerSPA(app, d)
	}

	// ต้องอยู่ท้ายสุด
	registerRedirect(app, d)

	return app
}
