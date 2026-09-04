package handler

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/middleware"
	"goshort/internal/model"
	"goshort/internal/repository"
)

type createLinkRequest struct {
	LongURL   string  `json:"long_url"`
	Alias     string  `json:"alias"`
	ExpiresAt *string `json:"expires_at"`
}

func registerLinks(app *fiber.App, d Deps) {
	app.Post("/api/links",
		middleware.RateLimit(d.Redis, "create", 20, time.Minute),
		func(c *fiber.Ctx) error {
			var req createLinkRequest
			if err := c.BodyParser(&req); err != nil {
				return fail(c, fiber.StatusBadRequest, "request body must be JSON")
			}

			var expiresAt *time.Time
			if req.ExpiresAt != nil && *req.ExpiresAt != "" {
				ts, err := time.Parse(time.RFC3339, *req.ExpiresAt)
				if err != nil {
					return fail(c, fiber.StatusBadRequest, "expires_at must be an RFC3339 timestamp")
				}
				expiresAt = &ts
			}

			if err := validateLongURL(req.LongURL, d.Cfg.BaseURL); err != nil {
				return fail(c, fiber.StatusBadRequest, err.Error())
			}

			if req.Alias == "" {
				link, err := d.links.Create(c.UserContext(), req.LongURL, expiresAt)
				if err != nil {
					return err
				}
				return c.Status(fiber.StatusCreated).JSON(linkCreated(d, link))
			}

			if err := validateAlias(req.Alias); err != nil {
				return fail(c, fiber.StatusBadRequest, err.Error())
			}
			if isReserved(req.Alias) {
				return fail(c, fiber.StatusConflict, "that alias is reserved, pick another one")
			}

			link, err := d.links.CreateWithAlias(c.UserContext(), req.Alias, req.LongURL, expiresAt)
			if errors.Is(err, repository.ErrCodeTaken) {
				return fail(c, fiber.StatusConflict, "that alias is already taken, pick another one")
			}
			if err != nil {
				return err
			}
			return c.Status(fiber.StatusCreated).JSON(linkCreated(d, link))
		})
}

func linkCreated(d Deps, l model.Link) fiber.Map {
	return fiber.Map{
		"code":      l.ShortCode,
		"short_url": d.Cfg.BaseURL + "/" + l.ShortCode,
		"long_url":  l.LongURL,
	}
}

func fail(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

// ตอบตัวเลขศูนย์เวลาอ่านแหล่งไม่ได้คือการบอกว่า "ไม่มีอะไรเกิดขึ้น" ซึ่งคนละเรื่องกับ
// "มองไม่เห็น" — หน้าจอเฝ้าระบบที่สับสนสองอย่างนี้แย่กว่าไม่มีหน้าจอเลย
func unreachableSource(c *fiber.Ctx, err error) error {
	slog.Warn("metrics source unreachable", "error", err)
	return fail(c, fiber.StatusServiceUnavailable, "cannot reach the instance being watched")
}
