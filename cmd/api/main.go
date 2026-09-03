package main

import (
	"log"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"goshort/internal/config"
	"goshort/internal/handler"
	"goshort/internal/model"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{TranslateError: true})
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

	app := handler.New(handler.Deps{DB: db, Redis: rdb, Cfg: cfg})

	log.Printf("goshort listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
