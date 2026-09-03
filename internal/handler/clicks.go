package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	"goshort/internal/model"
	"goshort/internal/worker"
)

// เก็บ IP ดิบไม่ได้ hash ตั้งแต่ก่อนเข้าคิว ไม่ใช่ตอนจะเขียนลง DB
// เพื่อไม่ให้มีจุดไหนในระบบถือ IP จริงไว้นานกว่าที่จำเป็น
func hashIP(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

// ค่าจาก c.Get() ชี้เข้า buffer ที่ fasthttp เอากลับไปใช้กับ request ถัดไปทันที
// ที่ handler จบ ต้อง copy ก่อนส่งเข้า channel ไม่งั้นข้อมูลที่ worker เขียนลง DB
// จะเป็นสตริงลูกผสมของหลาย request (ดู live_test.go)
func (d Deps) recordClick(c *fiber.Ctx, link model.Link) {
	e := worker.Event{
		LinkID:    link.ID,
		IPHash:    hashIP(c.IP()),
		UserAgent: utils.CopyString(c.Get(fiber.HeaderUserAgent)),
		Referrer:  utils.CopyString(c.Get(fiber.HeaderReferer)),
		At:        time.Now().UTC(),
	}

	if d.Cfg.SyncMode {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Clicks.WriteBatch(ctx, []worker.Event{e})
		return
	}

	d.Pool.Enqueue(e)
}
