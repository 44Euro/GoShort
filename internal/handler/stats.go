package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/cache"
	"goshort/internal/middleware"
	"goshort/internal/model"
	"goshort/internal/repository"
)

// aggregate เป็น GROUP BY ที่เปิดให้คนไม่ล็อกอินยิงได้ cache ไว้สั้น ๆ กันโดนถล่ม
func cached[T any](d Deps, ctx context.Context, key string, build func() (T, error)) (T, error) {
	if raw, err := d.Redis.Get(ctx, key).Bytes(); err == nil {
		var v T
		if json.Unmarshal(raw, &v) == nil {
			return v, nil
		}
	}
	v, err := build()
	if err != nil {
		return v, err
	}
	if raw, err := json.Marshal(v); err == nil {
		d.Redis.Set(ctx, key, raw, d.Cfg.AggregateCacheTTL)
	}
	return v, nil
}

type linkStats struct {
	Code      string                `json:"code"`
	LongURL   string                `json:"long_url"`
	Clicks    int64                 `json:"clicks"`
	Series    []repository.DayPoint `json:"series"`
	Referrers []repository.Referrer `json:"referrers"`
}

func registerStats(app *fiber.App, d Deps) {
	links := repository.NewLinkRepo(d.DB)
	stats := repository.NewStatsRepo(d.DB)

	app.Get("/api/links/:code/stats",
		middleware.RateLimit(d.Redis, "stats", 60, time.Minute),
		func(c *fiber.Ctx) error {
			link, err := links.ByCode(c.UserContext(), c.Params("code"))
			if err != nil {
				if repository.IsNotFound(err) {
					return fail(c, fiber.StatusNotFound, "no such link")
				}
				return err
			}

			out, err := cached(d, c.UserContext(), "agg:stats:"+link.ShortCode, func() (linkStats, error) {
				series, err := stats.DailyForLink(c.UserContext(), link.ID, repository.SeriesDays)
				if err != nil {
					return linkStats{}, err
				}
				refs, err := stats.Referrers(c.UserContext(), &link.ID, 6)
				if err != nil {
					return linkStats{}, err
				}
				return linkStats{
					Code: link.ShortCode, LongURL: link.LongURL,
					Clicks: link.ClickCount, Series: series, Referrers: refs,
				}, nil
			})
			if err != nil {
				return err
			}
			// click_count เปลี่ยนบ่อยกว่ารอบ cache ของ aggregate อ่านสดทับไป
			out.Clicks = link.ClickCount
			return c.JSON(out)
		})
}

func registerAdminAnalytics(g fiber.Router, d Deps) {
	links := repository.NewLinkRepo(d.DB)
	stats := repository.NewStatsRepo(d.DB)
	lc := cache.NewLinkCache(d.Redis, d.Cfg.CacheTTL)

	g.Get("/links/:code/analytics", func(c *fiber.Ctx) error {
		link, err := links.ByCode(c.UserContext(), c.Params("code"))
		if err != nil {
			if repository.IsNotFound(err) {
				return fail(c, fiber.StatusNotFound, "no such link")
			}
			return err
		}

		series, err := stats.DailyForLink(c.UserContext(), link.ID, repository.SeriesDays)
		if err != nil {
			return err
		}
		refs, err := stats.Referrers(c.UserContext(), &link.ID, 6)
		if err != nil {
			return err
		}
		unique, err := stats.UniqueVisitors(c.UserContext(), link.ID)
		if err != nil {
			return err
		}
		events, err := stats.RecentEvents(c.UserContext(), link.ID, 8)
		if err != nil {
			return err
		}

		cacheState := fiber.Map{"warm": false, "ttl_seconds": 0}
		if ttl, ok := lc.TTL(c.UserContext(), link.ShortCode); ok {
			cacheState = fiber.Map{"warm": true, "ttl_seconds": int(ttl.Seconds())}
		}

		days := float64(repository.SeriesDays)
		return c.JSON(fiber.Map{
			"code":       link.ShortCode,
			"long_url":   link.LongURL,
			"clicks":     link.ClickCount,
			"unique":     unique,
			"per_day":    float64(link.ClickCount) / days,
			"status":     linkStatus(link),
			"cache":      cacheState,
			"series":     series,
			"referrers":  refs,
			"events":     publicEvents(events),
			"created_at": link.CreatedAt.UTC().Format(time.RFC3339),
		})
	})

	g.Get("/dashboard/summary", func(c *fiber.Ctx) error {
		d.syncPoolCounters()
		s := d.Metrics.Summary()
		return c.JSON(fiber.Map{
			"cache_hit_rate":  s.CacheHitRate,
			"p99_redirect_ms": s.P99Millis,
			"queue_depth":     s.QueueDepth,
			"queue_capacity":  s.QueueCap,
			"dropped_events":  s.Dropped,
			"written_events":  s.Written,
			"total_clicks":    d.totalClicks(c),
		})
	})

	g.Get("/dashboard/overview", func(c *fiber.Ctx) error {
		type overview struct {
			Series    []repository.DayPoint `json:"series"`
			TopLinks  []fiber.Map           `json:"top_links"`
			Referrers []repository.Referrer `json:"referrers"`
		}

		out, err := cached(d, c.UserContext(), "agg:overview", func() (overview, error) {
			series, err := stats.DailyForAll(c.UserContext(), repository.SeriesDays)
			if err != nil {
				return overview{}, err
			}
			refs, err := stats.Referrers(c.UserContext(), nil, 6)
			if err != nil {
				return overview{}, err
			}
			all, err := links.All(c.UserContext())
			if err != nil {
				return overview{}, err
			}
			top := make([]fiber.Map, 0, 5)
			for i, l := range all {
				if i == 5 {
					break
				}
				top = append(top, fiber.Map{
					"rank": i + 1, "code": l.ShortCode,
					"long_url": l.LongURL, "clicks": l.ClickCount,
				})
			}
			return overview{Series: series, TopLinks: top, Referrers: refs}, nil
		})
		if err != nil {
			return err
		}
		return c.JSON(out)
	})
}

// ip_hash เต็มไม่ควรออกจากเซิร์ฟเวอร์ ตัดเหลือ 8 ตัวพอให้แยกคนได้ด้วยตา
func publicEvents(events []model.ClickEvent) []fiber.Map {
	out := make([]fiber.Map, 0, len(events))
	for _, e := range events {
		hash := e.IPHash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		ref := e.Referrer
		if ref == "" {
			ref = "(direct)"
		}
		out = append(out, fiber.Map{
			"time":       e.CreatedAt.UTC().Format("15:04:05"),
			"referrer":   ref,
			"user_agent": e.UserAgent,
			"ip_hash":    hash,
		})
	}
	return out
}
