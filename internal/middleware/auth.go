package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const SessionCookie = "goshort_session"

const sessionTTL = 24 * time.Hour

func IssueToken(secret, email string) (string, time.Time, error) {
	expires := time.Now().Add(sessionTTL)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   email,
		ExpiresAt: jwt.NewNumericDate(expires),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})
	signed, err := tok.SignedString([]byte(secret))
	return signed, expires, err
}

func parseToken(secret, raw string) (string, error) {
	// บังคับ HS256 ตรงนี้ ไม่งั้น token ที่ประกาศ alg=none จะผ่านการตรวจไปได้
	tok, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return "", err
	}
	claims, ok := tok.Claims.(*jwt.RegisteredClaims)
	if !ok || !tok.Valid || claims.Subject == "" {
		return "", fmt.Errorf("invalid token")
	}
	return claims.Subject, nil
}

func SessionCookieFor(token string, expires time.Time, secure bool) *fiber.Cookie {
	return &fiber.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	}
}

func ClearSessionCookie(secure bool) *fiber.Cookie {
	c := SessionCookieFor("", time.Now().Add(-time.Hour), secure)
	c.MaxAge = -1
	return c
}

func RequireAdmin(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		email, err := parseToken(secret, c.Cookies(SessionCookie))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "sign in first"})
		}
		c.Locals("admin_email", email)
		return c.Next()
	}
}
