package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"

	"goshort/internal/model"
	"goshort/internal/worker"
)

// เก็บ IP ดิบไม่ได้ hash ตั้งแต่ก่อนเข้าคิว ไม่ใช่ตอนจะเขียนลง DB
// เพื่อไม่ให้มีจุดไหนในระบบถือ IP จริงไว้นานกว่าที่จำเป็น
func hashIP(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

func (d Deps) recordClick(c *fiber.Ctx, link model.Link) {
	e := worker.Event{
		LinkID:    link.ID,
		IPHash:    hashIP(c.IP()),
		UserAgent: c.Get(fiber.HeaderUserAgent),
		Referrer:  c.Get(fiber.HeaderReferer),
		At:        time.Now().UTC(),
	}

	if d.Cfg.SyncMode {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Clicks.WriteBatchNow(ctx, e)
		return
	}

	d.Pool.Enqueue(e)
}
