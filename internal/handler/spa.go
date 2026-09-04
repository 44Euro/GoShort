package handler

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

// path ที่ SPA เป็นเจ้าของ ลงทะเบียนก่อน /:code เพื่อไม่ให้ redirect handler กิน
var spaRoutes = []string{
	"/", "/login", "/s", "/s/:code",
	"/admin", "/admin/analytics", "/admin/links", "/admin/links/:code",
}

func registerSPA(app *fiber.App, assets fs.FS) {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return
	}

	shell := func(status int) fiber.Handler {
		return func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Status(status).Send(index)
		}
	}

	app.Use("/assets", filesystem.New(filesystem.Config{Root: http.FS(assets), PathPrefix: "assets"}))

	for _, f := range []string{"favicon.ico", "robots.txt", "vite.svg"} {
		if body, err := fs.ReadFile(assets, f); err == nil {
			app.Get("/"+f, func(c *fiber.Ctx) error { return c.Send(body) })
		}
	}

	for _, r := range spaRoutes {
		app.Get(r, shell(fiber.StatusOK))
	}
}

// เสิร์ฟ SPA พร้อม status 404 จริง ไม่ใช่ 200 — เครื่องมือภายนอกต้องเห็นว่าลิงก์ตายแล้ว
func spaNotFound(c *fiber.Ctx, index []byte) error {
	if index == nil || strings.HasPrefix(c.Path(), "/api/") {
		return fail(c, fiber.StatusNotFound, "no such link")
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Status(fiber.StatusNotFound).Send(index)
}
