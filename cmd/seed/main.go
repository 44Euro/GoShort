package main

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"goshort/internal/config"
	"goshort/internal/model"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Println("seed: schema ready")
}
