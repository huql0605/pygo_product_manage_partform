package router

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/analytics"
	"github.com/huql0605/go_api/internal/config"
	"github.com/huql0605/go_api/internal/mapping"
	"github.com/huql0605/go_api/internal/middleware"
	"github.com/huql0605/go_api/internal/product"
	"github.com/huql0605/go_api/internal/raw"
	"github.com/huql0605/go_api/internal/sale"
	"gorm.io/gorm"
)

func New(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.New()
	base_url := cfg.Python.BaseURL
	pyClient := analytics.NewClient(base_url)
	r.HandleMethodNotAllowed = true

	repo := product.NewRepository(db)
	svc := product.NewService(repo)

	salerepo := sale.NewRepository(db)
	salesvc := sale.NewService(salerepo, repo)

	rawRepo := raw.NewRepository(db)
	rawSvc := raw.NewService(rawRepo)

	mappingRepo := mapping.NewRepository(db)
	mappingSvc := mapping.NewService(mappingRepo, rawRepo, salesvc)

	mh := mapping.NewHandler(mappingSvc)
	rh := raw.NewHandler(rawSvc)
	sh := sale.NewHandler(salesvc)
	ph := product.NewHandler(svc)
	ah := analytics.NewHandler(pyClient)

	// 顺序很重要：先生成/取出 RequestID，Logger 才能打出来。
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.GET("/api/products", ph.List)
	r.POST("/api/sales/batch", sh.CreateBatch)
	r.POST("/api/products", ph.Create)
	r.POST("/api/imports", rh.CreateImport)
	r.POST("/api/imports/:id/transform", mh.Transform)
	r.GET("/api/imports/:id/columns", rh.ListColumns)
	r.POST("/api/mappings", mh.Upsert)
	r.GET("/api/mappings", mh.List)
	r.GET("/api/analytics/sales", ah.Sales)
	r.GET("/api/analytics/top-products", ah.TopProducts)
	r.GET("/api/analytics/brands", ah.Brands)
	r.GET("/api/analytics/monthly-trend", ah.MonthlyTrend)
	return r
}

func MustMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(
		&product.Product{}, &sale.Sales{},
		&raw.ImportBatch{}, &raw.RawRecord{},
		&mapping.FieldMapping{},
	); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}
}
