package handler

import (
	"github.com/gofiber/fiber/v2"

	"goshort/internal/middleware"
	"goshort/internal/repository"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func registerAdmin(app *fiber.App, d Deps) {
	admins := repository.NewAdminRepo(d.DB)
	secure := !d.Cfg.Dev

	app.Post("/api/admin/login", func(c *fiber.Ctx) error {
		var req loginRequest
		if err := c.BodyParser(&req); err != nil {
			return fail(c, fiber.StatusBadRequest, "request body must be JSON")
		}

		admin, ok := admins.Authenticate(c.Context(), req.Email, req.Password)
		if !ok {
			return fail(c, fiber.StatusUnauthorized, "email or password is wrong")
		}

		token, expires, err := middleware.IssueToken(d.Cfg.JWTSecret, admin.Email)
		if err != nil {
			return err
		}
		c.Cookie(middleware.SessionCookieFor(token, expires, secure))
		return c.JSON(fiber.Map{"email": admin.Email})
	})

	app.Post("/api/admin/logout", func(c *fiber.Ctx) error {
		c.Cookie(middleware.ClearSessionCookie(secure))
		return c.JSON(fiber.Map{"ok": true})
	})

	guarded := app.Group("/api/admin", middleware.RequireAdmin(d.Cfg.JWTSecret))

	guarded.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"email": c.Locals("admin_email")})
	})

	registerAdminLinks(guarded, d)
}
