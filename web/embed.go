package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist คืน dist/ ที่ตัด prefix ออกแล้ว ถ้ายังไม่ได้ build จะได้ FS ว่างซึ่ง
// ทำให้ทุก path หลุดไปที่ SPA fallback แทนที่จะทำให้ทั้งโปรเซสพัง
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return dist
	}
	return sub
}
