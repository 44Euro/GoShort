package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	BaseURL     string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string

	AdminEmail    string
	AdminPassword string

	// ปิดแล้ว route ของ admin จะไม่ถูกลงทะเบียนเลย ไม่ใช่ลงทะเบียนแล้วกันด้วย guard
	// instance ที่รับ traffic สาธารณะจึงไม่มีเส้นให้ยิงตั้งแต่แรก
	// ปริยายเป็นเปิด เพราะการรัน instance เดียวทำทั้งสองหน้าที่คือกรณีปกติของระบบขนาดนี้
	AdminEnabled bool

	// text สำหรับอ่านตอน dev, json สำหรับให้เครื่องดูด
	LogFormat string

	// ตัวเลขวัดผลเป็นของแต่ละโปรเซส คอนโซลที่อยู่คนละโปรเซสกับตัวที่รับ redirect
	// จึงต้องอ่านของ instance ที่มันเฝ้า ไม่ใช่ของตัวเอง ไม่ตั้ง = อ่านของตัวเอง
	MetricsSourceURL string

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	ClickBufferSize   int
	ClickWorkers      int
	ClickBatchSize    int
	ClickFlushEvery   time.Duration
	ShutdownTimeout   time.Duration
	CacheTTL          time.Duration
	AggregateCacheTTL time.Duration

	// SyncMode ปิด cache และเขียน click event ในคำขอเดียวกัน ใช้วัด baseline
	// ก่อน optimize ด้วย binary ตัวเดียวกัน ไม่ต้อง build ใหม่ (ดู load-test/)
	SyncMode bool

	// เชื่อ X-Forwarded-For เฉพาะเมื่อรู้ว่ามี proxy จริงคั่นอยู่ ไม่งั้นใครก็
	// ปลอม header นี้เพื่อรีเซ็ตโควตา rate limit ของตัวเองได้
	TrustProxy bool
}

// อ่านจาก scheme ของ BASE_URL แทนที่จะให้ตั้ง flag แยก — ตั้ง Secure บน http
// ทำให้ Safari ทิ้ง cookie เงียบ ๆ และ docker-compose ก็รันบน http://localhost
func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}

func Load() (Config, error) {
	c := Config{
		Port:              env("PORT", "8080"),
		BaseURL:           strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		DatabaseURL:       env("DATABASE_URL", ""),
		RedisURL:          env("REDIS_URL", ""),
		JWTSecret:         env("JWT_SECRET", ""),
		AdminEmail:        env("ADMIN_EMAIL", "admin@goshort.dev"),
		AdminPassword:     env("ADMIN_PASSWORD", ""),
		AdminEnabled:      envBool("ADMIN_ENABLED", true),
		LogFormat:         env("LOG_FORMAT", "text"),
		MetricsSourceURL:  env("METRICS_SOURCE_URL", ""),
		DBMaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 25),
		ClickBufferSize:   envInt("CLICK_BUFFER_SIZE", 1000),
		ClickWorkers:      envInt("CLICK_WORKERS", 8),
		ClickBatchSize:    envInt("CLICK_BATCH_SIZE", 50),
		ClickFlushEvery:   envDuration("CLICK_FLUSH_INTERVAL", 2*time.Second),
		ShutdownTimeout:   envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		CacheTTL:          envDuration("CACHE_TTL", time.Hour),
		AggregateCacheTTL: envDuration("AGGREGATE_CACHE_TTL", 60*time.Second),
		SyncMode:          envBool("GOSHORT_SYNC_MODE", false),
		TrustProxy:        envBool("TRUST_PROXY", false),
	}

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return c, fmt.Errorf("JWT_SECRET is required")
	}
	if c.ClickBufferSize < 1 || c.ClickWorkers < 1 || c.ClickBatchSize < 1 {
		return c, fmt.Errorf("click buffer size, worker count and batch size must all be at least 1")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}
