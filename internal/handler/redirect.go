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
	app.Get("/:code", func(c *fiber.Ctx) error {
		started := time.Now()
		defer func() { d.Metrics.ObserveRedirect(time.Since(started)) }()

		code := c.Params("code")

		if !d.Cfg.SyncMode {
			if e, ok := d.cache.Get(c.UserContext(), code); ok && !e.Expired(time.Now()) {
				d.Metrics.CacheHit()
				d.recordClick(c, model.Link{ID: e.ID})
				return c.Redirect(e.LongURL, fiber.StatusFound)
			}
			d.Metrics.CacheMiss()
		}

		link, err := d.links.ByCode(c.UserContext(), code)
		if err != nil {
			if repository.IsNotFound(err) {
				return spaNotFound(c, d.index)
			}
			return err
		}
		if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
			return spaNotFound(c, d.index)
		}

		if !d.Cfg.SyncMode {
			d.cache.Set(c.UserContext(), code, cache.Entry{
				ID: link.ID, LongURL: link.LongURL, ExpiresAt: link.ExpiresAt,
			})
		}
		d.recordClick(c, link)

		return c.Redirect(link.LongURL, fiber.StatusFound)
	})
}
