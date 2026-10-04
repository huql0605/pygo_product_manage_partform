package router

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/analytics"
	"github.com/huql0605/go_api/internal/config"
	"github.com/huql0605/go_api/internal/product"
	"gorm.io/gorm"
)

func New(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()
	base_url := cfg.Python.BaseURL
	pyClient := analytics.NewClient(base_url)
	r.HandleMethodNotAllowed = true
	repo := product.NewRepository(db)
	svc := product.NewService(repo)
	ph := product.NewHandler(svc)
	ah := analytics.NewHandler(pyClient)

	r.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.GET("/api/products", ph.List)
	r.POST("/api/products", ph.Create)
	r.GET("api/analytics/sales", ah.Sales)
	r.GET("/api/analytics/top-products", ah.TopProducts)
	r.GET("/api/analytics/brands", ah.Brands)
	r.GET("/api/analytics/monthly-trend", ah.MonthlyTrend)
	return r
}

func MustMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&product.Product{}); err != nil {
		log.Fatalf("migrate failed :%v", err)
	}
}
