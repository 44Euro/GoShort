package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"goshort/internal/config"
	"goshort/internal/handler"
	"goshort/internal/metrics"
	"goshort/internal/model"
	"goshort/internal/repository"
	"goshort/internal/worker"
	"goshort/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.Open(cfg.DatabaseURL, repository.PoolConfig{
		MaxOpen:     cfg.DBMaxOpenConns,
		MaxIdle:     cfg.DBMaxIdleConns,
		MaxLifetime: 30 * time.Minute,
		MaxIdleTime: 5 * time.Minute,
	})
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opts)

	clicks := repository.NewClickStore(db)
	pool := worker.New(clicks, worker.Config{
		Workers:    cfg.ClickWorkers,
		Buffer:     cfg.ClickBufferSize,
		BatchSize:  cfg.ClickBatchSize,
		FlushEvery: cfg.ClickFlushEvery,
	})
	pool.Start()

	m := metrics.New()
	m.TrackQueue(
		func() float64 { return float64(pool.Depth()) },
		func() float64 { return float64(pool.Capacity()) },
	)

	app := handler.New(handler.Deps{
		Assets: web.Dist(),
		DB:     db, Redis: rdb, Cfg: cfg,
		Pool: pool, Clicks: clicks, Metrics: m,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		mode := "async + cache"
		if cfg.SyncMode {
			mode = "SYNC MODE (no cache, synchronous writes)"
		}
		log.Printf("goshort listening on :%s — %s, %d workers, buffer %d",
			cfg.Port, mode, cfg.ClickWorkers, cfg.ClickBufferSize)
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	// ลำดับสำคัญ: หยุดรับ request ใหม่ก่อน แล้วค่อยไล่ของค้างในคิว
	// สลับลำดับเมื่อไหร่ก็จะมี click ใหม่ไหลเข้าคิวหลังปิด worker แล้วหายไปเงียบ ๆ
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	if err := pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("click writer shutdown: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	_ = rdb.Close()

	log.Println("bye")
}
