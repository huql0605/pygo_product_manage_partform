package main

import (
	"log"
	"net/http"

	"github.com/huql0605/go_api/internal/router"
)

func main() {
	r:=router.New()
	log.Println("go api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080",r))
}