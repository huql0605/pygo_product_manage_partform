package router

import (
	"net/http"

	"github.com/huql0605/go_api/internal/product"
)

func New() http.Handler {
	mux := http.NewServeMux()
	store := product.NewStore()
	ph := product.NewHandler(store)

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
