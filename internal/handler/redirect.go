package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/cache"
	"goshort/internal/repository"
)

// ต้องลงทะเบียนเป็น route สุดท้ายเสมอ ดู New()
func registerRedirect(app *fiber.App, d Deps) {
	links := repository.NewLinkRepo(d.DB)
	lc := cache.NewLinkCache(d.Redis, d.Cfg.CacheTTL)

	app.Get("/:code", func(c *fiber.Ctx) error {
		code := c.Params("code")

		if !d.Cfg.SyncMode {
			if longURL, ok := lc.Get(c.Context(), code); ok {
				return c.Redirect(longURL, fiber.StatusFound)
			}
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
			lc.Set(c.Context(), code, link.LongURL)
		}

		return c.Redirect(link.LongURL, fiber.StatusFound)
	})
}
