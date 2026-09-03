package handler

import (
	"io/fs"
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/cache"
	"goshort/internal/model"
	"goshort/internal/repository"
)

// ต้องลงทะเบียนเป็น route สุดท้ายเสมอ ดู New()
func registerRedirect(app *fiber.App, d Deps) {
	links := repository.NewLinkRepo(d.DB)
	lc := cache.NewLinkCache(d.Redis, d.Cfg.CacheTTL)

	var index []byte
	if d.Assets != nil {
		index, _ = fs.ReadFile(d.Assets, "index.html")
	}

	app.Get("/:code", func(c *fiber.Ctx) error {
		started := time.Now()
		defer func() { d.Metrics.ObserveRedirect(time.Since(started)) }()

		code := c.Params("code")

		if !d.Cfg.SyncMode {
			if e, ok := lc.Get(c.UserContext(), code); ok {
				d.Metrics.CacheHit()
				d.recordClick(c, model.Link{ID: e.ID})
				return c.Redirect(e.LongURL, fiber.StatusFound)
			}
			d.Metrics.CacheMiss()
		}

		link, err := links.ByCode(c.UserContext(), code)
		if err != nil {
			if repository.IsNotFound(err) {
				return spaNotFound(c, index)
			}
			return err
		}
		if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
			return spaNotFound(c, index)
		}

		if !d.Cfg.SyncMode {
			lc.Set(c.UserContext(), code, cache.Entry{ID: link.ID, LongURL: link.LongURL})
		}
		d.recordClick(c, link)

		return c.Redirect(link.LongURL, fiber.StatusFound)
	})
}
