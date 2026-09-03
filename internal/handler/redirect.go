package handler

import (
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

	app.Get("/:code", func(c *fiber.Ctx) error {
		started := time.Now()
		defer func() { d.Metrics.ObserveRedirect(time.Since(started)) }()

		code := c.Params("code")

		if !d.Cfg.SyncMode {
			if e, ok := lc.Get(c.Context(), code); ok {
				d.Metrics.CacheHit()
				d.recordClick(c, model.Link{ID: e.ID})
				return c.Redirect(e.LongURL, fiber.StatusFound)
			}
			d.Metrics.CacheMiss()
		}

		link, err := links.ByCode(c.Context(), code)
		if err != nil {
			if repository.IsNotFound(err) {
				return fail(c, fiber.StatusNotFound, "no such link")
			}
			return err
		}
		if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
			return fail(c, fiber.StatusNotFound, "no such link")
		}

		if !d.Cfg.SyncMode {
			lc.Set(c.Context(), code, cache.Entry{ID: link.ID, LongURL: link.LongURL})
		}
		d.recordClick(c, link)

		return c.Redirect(link.LongURL, fiber.StatusFound)
	})
}
