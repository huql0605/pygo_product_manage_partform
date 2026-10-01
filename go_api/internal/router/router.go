package router

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/product"
	"gorm.io/gorm"
)

func New(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.HandleMethodNotAllowed = true
	repo := product.NewRepository(db)
	svc := product.NewService(repo)
	ph := product.NewHandler(svc)

	r.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.GET("/api/products", ph.List)
	r.POST("/api/products", ph.Create)
	return r
}

func MustMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&product.Product{}); err != nil {
		log.Fatalf("migrate failed :%v", err)
	}
}
