package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

const RequestIDHeader = "X-Request-ID"

const requestIDLocal = "request_id"

// ค่าที่ได้จาก c.Get ชี้เข้า buffer ที่ fasthttp เอากลับไปใช้ซ้ำทันทีที่ handler จบ
// ต้องคัดลอกก่อนเก็บ เป็นบั๊กตัวเดียวกับที่เคยทำให้ referrer ใน Postgres เป็นสตริงต่อกัน
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := utils.CopyString(c.Get(RequestIDHeader))
		// ค่าจากภายนอกไหลลง log ปล่อยให้ยาวเท่าไหร่ก็ได้คือปล่อยให้ใครก็ถมไฟล์ log ได้
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		c.Locals(requestIDLocal, id)
		c.Set(RequestIDHeader, id)
		return c.Next()
	}
}

func RequestIDOf(c *fiber.Ctx) string {
	id, _ := c.Locals(requestIDLocal).(string)
	return id
}

func AccessLog() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		// ErrorHandler ของ Fiber ทำงานเหนือ middleware ชั้นนี้ สถานะบน response
		// จึงยังไม่ใช่ตัวสุดท้ายเมื่อ handler คืน error มา ต้องอ่านจาก error เอง
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				status = fe.Code
			}
		}

		attrs := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000,
			"request_id", RequestIDOf(c),
		}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		slog.Info("request", attrs...)
		return err
	}
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}
