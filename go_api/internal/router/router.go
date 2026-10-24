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
	"github.com/huql0605/go_api/internal/summary"
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

	// summary 要先于 raw / mapping 创建：删除批次与转换批次都需要在完成后重建汇总层。
	summaryRepo := summary.NewRepository(db)
	summarySvc := summary.NewService(summaryRepo)

	rawRepo := raw.NewRepository(db)
	rawSvc := raw.NewService(rawRepo).WithSummary(summarySvc)

	mappingRepo := mapping.NewRepository(db)
	mappingSvc := mapping.NewService(mappingRepo, rawRepo, salesvc).WithSummary(summarySvc)

	mh := mapping.NewHandler(mappingSvc)
	rh := raw.NewHandler(rawSvc)
	sh := sale.NewHandler(salesvc)
	ph := product.NewHandler(svc)
	ah := analytics.NewHandler(pyClient)
	smh := summary.NewHandler(summarySvc)

	// 顺序很重要：先生成/取出 RequestID，Logger 才能打出来。
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.GET("/api/products", ph.List) //获取sales表所有商品
	r.POST("/api/sales/batch", sh.CreateBatch)
	r.POST("/api/products", ph.Create)
	r.GET("/api/imports", rh.ListImports)
	r.POST("/api/imports", rh.CreateImport)
	r.DELETE("/api/imports/:id", rh.DeleteImport)
	r.POST("/api/imports/:id/transform", mh.Transform)
	r.GET("/api/imports/:id/columns", rh.ListColumns)
	r.GET("/api/imports/:id", rh.ShowImportsCount)
	r.POST("/api/mappings", mh.Upsert)
	r.GET("/api/mappings", mh.List)
	r.POST("/api/summary/refresh", smh.Refresh)
	r.GET("/api/summary/verify", smh.Verify)
	r.GET("/api/analytics/sales", ah.Sales)
	r.GET("/api/analytics/top-products", ah.TopProducts)
	r.GET("/api/analytics/brands", ah.Brands)
	r.GET("/api/analytics/monthly-trend", ah.MonthlyTrend)
	r.GET("/api/analytics/attributes", ah.Attributes)
	return r
}

func MustMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(
		&product.Product{}, &sale.Sales{},
		&raw.ImportBatch{}, &raw.RawRecord{},
		&mapping.FieldMapping{},
		&summary.SalesSummary{},
	); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}

	// sales.attrs 的 GIN 索引。属性不进汇总层，聚合直接扫明细，靠它收敛扫描范围。
	// 用 jsonb_path_ops：索引体积更小，且支持 @> 与 ? 两种操作符。
	if err := db.Exec(
		`CREATE INDEX IF NOT EXISTS idx_sales_attrs ON sales USING gin (attrs jsonb_path_ops)`,
	).Error; err != nil {
		log.Fatalf("create attrs index failed: %v", err)
	}
}
