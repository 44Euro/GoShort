package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/repository"
)

// ต้องลงทะเบียนเป็น route สุดท้ายเสมอ ดู New()
func registerRedirect(app *fiber.App, d Deps) {
	links := repository.NewLinkRepo(d.DB)

	app.Get("/:code", func(c *fiber.Ctx) error {
		code := c.Params("code")

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

		return c.Redirect(link.LongURL, fiber.StatusFound)
	})
}
