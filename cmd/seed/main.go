package main

import (
	"context"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"goshort/internal/config"
	"goshort/internal/model"
	"goshort/internal/repository"
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

	if cfg.AdminPassword == "" {
		log.Fatal("ADMIN_PASSWORD is required to seed the administrator")
	}
	if err := repository.NewAdminRepo(db).Upsert(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	log.Printf("seeded administrator %s", cfg.AdminEmail)
}
