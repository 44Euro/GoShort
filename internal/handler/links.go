package handler

import (
	"errors"
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
	links := repository.NewLinkRepo(d.DB)

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
				link, err := links.Create(c.Context(), req.LongURL, expiresAt)
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

			link, err := links.CreateWithAlias(c.Context(), req.Alias, req.LongURL, expiresAt)
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
