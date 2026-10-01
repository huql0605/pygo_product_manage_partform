package router

import (
	"log"
	"net/http"

	"github.com/huql0605/go_api/internal/product"
	"gorm.io/gorm"
)

func New(db *gorm.DB) http.Handler {
	mux := http.NewServeMux()
	repo := product.NewRepository(db)
	svc := product.NewService(repo)
	ph := product.NewHandler(svc)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ph.List(w, r)
		case http.MethodPost:
			ph.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func MustMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&product.Product{}); err != nil {
		log.Fatalf("migrate failed :%v", err)
	}
}
