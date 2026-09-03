package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/cache"
	"goshort/internal/model"
	"goshort/internal/repository"
)

func registerAdminLinks(g fiber.Router, d Deps) {
	links := repository.NewLinkRepo(d.DB)
	stats := repository.NewStatsRepo(d.DB)
	lc := cache.NewLinkCache(d.Redis, d.Cfg.CacheTTL)

	g.Get("/links", func(c *fiber.Ctx) error {
		all, err := links.All(c.UserContext())
		if err != nil {
			return err
		}
		series, err := stats.DailyForEachLink(c.UserContext(), repository.SeriesDays)
		if err != nil {
			return err
		}

		rows := make([]fiber.Map, 0, len(all))
		for _, l := range all {
			spark := series[l.ID]
			if spark == nil {
				spark = make([]int64, repository.SeriesDays)
			}
			rows = append(rows, fiber.Map{
				"code":         l.ShortCode,
				"long_url":     l.LongURL,
				"clicks":       l.ClickCount,
				"status":       linkStatus(l),
				"custom_alias": l.CustomAlias,
				"created_at":   l.CreatedAt.UTC().Format(time.RFC3339),
				"last_14_days": spark,
			})
		}
		return c.JSON(fiber.Map{"links": rows})
	})

	g.Delete("/links/:code", func(c *fiber.Ctx) error {
		code := c.Params("code")
		if err := links.Delete(c.UserContext(), code); err != nil {
			if repository.IsNotFound(err) {
				return fail(c, fiber.StatusNotFound, "no such link")
			}
			return err
		}
		// ต้องล้าง cache ทันที ไม่ใช่รอ TTL ไม่งั้นลิงก์ที่ลบแล้วยังเด้งได้อีกชั่วโมง
		_ = lc.Invalidate(c.UserContext(), code)
		return c.JSON(fiber.Map{"deleted": code})
	})

	g.Post("/links/:code/invalidate-cache", func(c *fiber.Ctx) error {
		code := c.Params("code")
		if _, err := links.ByCode(c.UserContext(), code); err != nil {
			if repository.IsNotFound(err) {
				return fail(c, fiber.StatusNotFound, "no such link")
			}
			return err
		}
		if err := lc.Invalidate(c.UserContext(), code); err != nil {
			return err
		}
		return c.JSON(fiber.Map{"invalidated": code})
	})
}

func linkStatus(l model.Link) string {
	if l.ExpiresAt != nil && l.ExpiresAt.Before(time.Now()) {
		return "expired"
	}
	return "active"
}
