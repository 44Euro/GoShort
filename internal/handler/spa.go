package handler

import (
	"bytes"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

// path ที่ SPA เป็นเจ้าของ ลงทะเบียนก่อน /:code เพื่อไม่ให้ redirect handler กิน
var publicSPARoutes = []string{"/", "/s", "/s/:code"}

// บน instance ที่ไม่ได้ทำหน้าที่ admin path พวกนี้ต้องไม่ถูกจอง จะได้ตกไปที่ /:code
// แล้วได้ 404 จริง — บน host นั้นหน้าเหล่านี้ไม่มีอยู่
var adminSPARoutes = []string{
	"/login", "/admin", "/admin/analytics", "/admin/links", "/admin/links/:code",
}

const adminRolePlaceholder = "__GOSHORT_ADMIN__"

// SPA ต้องรู้บทบาทของ instance ตั้งแต่ไบต์แรกที่ได้รับ ไม่งั้นต้องยิง API ถามบนหน้า
// ที่ต้องเร็วที่สุด แทนที่ทีเดียวตอน boot ต้นทุนต่อ request จึงเป็นศูนย์
func spaShell(assets fs.FS, adminEnabled bool) []byte {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil
	}
	role := []byte("0")
	if adminEnabled {
		role = []byte("1")
	}
	return bytes.ReplaceAll(index, []byte(adminRolePlaceholder), role)
}

func registerSPA(app *fiber.App, d Deps) {
	if d.index == nil {
		return
	}
	assets, index, adminEnabled := d.Assets, d.index, d.Cfg.AdminEnabled

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

	for _, r := range publicSPARoutes {
		app.Get(r, shell(fiber.StatusOK))
	}
	if !adminEnabled {
		return
	}
	for _, r := range adminSPARoutes {
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
