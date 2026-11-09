package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/huql0605/go_api/internal/config"
	"github.com/huql0605/go_api/internal/database"
	"github.com/huql0605/go_api/internal/router"
	"github.com/huql0605/go_api/internal/summary"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("./.env")
	cfg, err := config.Load("configs/configs.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	if cfg.Import.SigningKey == "" {
		log.Fatal("IMPORT_SIGNING_KEY 未配置")
	}
	db, err := database.NewPostgres(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	router.MustMigrate(db)

	worker := summary.NewWorker(summary.NewRepository(db), time.Second)
	go worker.Run(context.Background())

	r := router.New(db, cfg)
	addr := ":" + strconv.Itoa(cfg.Server.Port)
	log.Printf("go api listen on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
