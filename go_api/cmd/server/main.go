package main

import (
	"log"
	"net/http"

	"github.com/huql0605/go_api/internal/database"
	"github.com/huql0605/go_api/internal/router"
)

func main() {
	db, err := database.NewPostgres(
		"host=localhost user=leohu password=hu0605 dbname=db1 port=5432 sslmode=disable",
	)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	router.MustMigrate(db)
	r := router.New(db)
	log.Println("go api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
