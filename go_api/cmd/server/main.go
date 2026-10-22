package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/huql0605/go_api/internal/config"
	"github.com/huql0605/go_api/internal/database"
	"github.com/huql0605/go_api/internal/router"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("./.env")
	cfg, err := config.Load("configs/configs.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	db, err := database.NewPostgres(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	router.MustMigrate(db)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_attrs
         ON sales USING gin (attrs jsonb_path_ops)`)
	r := router.New(db, cfg)
	addr := ":" + strconv.Itoa(cfg.Server.Port)
	log.Printf("go api listen on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
