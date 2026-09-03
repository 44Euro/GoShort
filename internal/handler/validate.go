package handler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// alias เหล่านี้ชนกับ path จริงของแอป ปล่อยให้จองได้ = ยิงลิงก์ทับหน้า /admin ได้
var reservedCodes = map[string]bool{
	"api": true, "admin": true, "health": true, "metrics": true,
	"login": true, "s": true, "assets": true,
	"favicon.ico": true, "robots.txt": true,
}

var aliasShape = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,12}$`)

func validateAlias(alias string) error {
	if !aliasShape.MatchString(alias) {
		return fmt.Errorf("alias must be 1-12 characters of letters, digits, hyphen or underscore")
	}
	return nil
}

func isReserved(alias string) bool {
	return reservedCodes[strings.ToLower(alias)] || strings.HasPrefix(alias, "_")
}

func validateLongURL(raw, baseURL string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("long_url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("long_url is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("long_url must start with http:// or https://")
	}
	if u.Host == "" {
		return fmt.Errorf("long_url is missing a host")
	}
	if base, err := url.Parse(baseURL); err == nil && strings.EqualFold(u.Host, base.Host) {
		return fmt.Errorf("long_url cannot point back at goshort")
	}
	return nil
}
