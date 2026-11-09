package summary

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/huql0605/go_api/internal/sale"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 集成测试：需要一个可写的 PostgreSQL 测试库。
//
//	TEST_DATABASE_URL='host=localhost user=leohu password=xxx dbname=db1_test port=5432 sslmode=disable' \
//	  go test ./internal/summary -v
//
// 未设置时自动跳过，保证 go test ./... 在无数据库环境下仍然通过。
// 为防误删开发数据，库名必须包含 test，否则同样跳过。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 TEST_DATABASE_URL，跳过集成测试")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}

	var dbName string
	if err := db.Raw("SELECT current_database()").Scan(&dbName).Error; err != nil {
		t.Fatalf("读取当前库名失败: %v", err)
	}
	if !strings.Contains(dbName, "test") {
		t.Skipf("库名 %q 不含 test，为避免误删数据已跳过", dbName)
	}

	if err := db.Exec("DROP TABLE IF EXISTS dws_sales, sales, summary_refresh_tasks CASCADE").Error; err != nil {
		t.Fatalf("清理旧表失败: %v", err)
	}
	if err := db.AutoMigrate(&sale.Sales{}, &SalesSummary{}, &RefreshTask{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS dws_sales, sales, summary_refresh_tasks CASCADE")
	})
	return db
}

func seedSales(t *testing.T, db *gorm.DB, rows ...sale.Sales) {
	t.Helper()
	for i := range rows {
		if len(rows[i].Attrs) == 0 {
			rows[i].Attrs = json.RawMessage("{}")
		}
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("插入明细失败: %v", err)
	}
}

// 同一个 grain（product_id + productor + name + month）由多个批次贡献时，
// 汇总层必须把它们合并成一行，并且各度量按可加规则累加。
func TestRefreshAll_AggregatesByGrain(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	b1, b2 := uint(1), uint(2)
	seedSales(t, db,
		sale.Sales{ProductID: 1, BatchID: &b1, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(100), Sales: 10, Month: "2024-01"},
		sale.Sales{ProductID: 1, BatchID: &b2, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(300), Sales: 5, Month: "2024-01"},
		sale.Sales{ProductID: 1, BatchID: &b1, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(200), Sales: 7, Month: "2024-02"},
	)

	n, err := repo.RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll 失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("期望 2 个 grain，实际 %d", n)
	}

	var got SalesSummary
	if err := db.Where("product_id = ? AND month = ?", 1, "2024-01").First(&got).Error; err != nil {
		t.Fatalf("读取汇总行失败: %v", err)
	}

	if got.TotalSales != 15 {
		t.Errorf("total_sales 期望 15，实际 %d", got.TotalSales)
	}
	if got.TotalAmount.Cmp(decimal.NewFromInt(2500)) != 0 {
		t.Errorf("total_amount 期望 10*100+5*300=2500，实际 %v", got.TotalAmount)
	}
	if got.PriceSum.Cmp(decimal.NewFromInt(400)) != 0 {
		t.Errorf("price_sum 期望 400，实际 %v", got.PriceSum)
	}
	if got.RecordCount != 2 {
		t.Errorf("record_count 期望 2，实际 %d", got.RecordCount)
	}
	if got.BatchCount != 2 {
		t.Errorf("batch_count 期望 2（两个批次），实际 %d", got.BatchCount)
	}
	if got.MinPrice.Cmp(decimal.NewFromInt(100)) != 0 || got.MaxPrice.Cmp(decimal.NewFromInt(300)) != 0 {
		t.Errorf("min/max price 期望 100/300，实际 %v/%v", got.MinPrice, got.MaxPrice)
	}
}

// 重复刷新必须幂等：行数与内容都不该变化。
func TestRefreshAll_IsIdempotent(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	seedSales(t, db,
		sale.Sales{ProductID: 1, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(100), Sales: 10, Month: "2024-01"},
	)

	first, err := repo.RefreshAll()
	if err != nil {
		t.Fatalf("第一次刷新失败: %v", err)
	}
	second, err := repo.RefreshAll()
	if err != nil {
		t.Fatalf("第二次刷新失败: %v", err)
	}
	if first != second {
		t.Fatalf("两次刷新行数不一致: %d vs %d", first, second)
	}

	diffs, err := repo.Verify()
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("期望无差异，实际 %+v", diffs)
	}
}

// 汇总行被手工改坏时，Verify 必须能发现，并且双向 EXCEPT 会各报一条。
func TestVerify_DetectsDrift(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	seedSales(t, db,
		sale.Sales{ProductID: 1, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(100), Sales: 10, Month: "2024-01"},
	)
	if _, err := repo.RefreshAll(); err != nil {
		t.Fatalf("刷新失败: %v", err)
	}

	if err := db.Model(&SalesSummary{}).
		Where("product_id = ? AND month = ?", 1, "2024-01").
		Update("total_sales", 999).Error; err != nil {
		t.Fatalf("篡改汇总行失败: %v", err)
	}

	diffs, err := repo.Verify()
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("期望 2 条差异（明细侧 + 汇总侧各一条），实际 %d: %+v", len(diffs), diffs)
	}

	sides := map[string]int{}
	for _, d := range diffs {
		sides[d.Side]++
	}
	if sides["detail_only"] != 1 || sides["summary_only"] != 1 {
		t.Fatalf("差异方向不符合预期: %+v", sides)
	}
}

// 删除同一 grain 的最后一个批次后，汇总行必须消失。
func TestRefreshGrains_DeleteLastContributionRemovesRow(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	b := uint(1)
	seedSales(t, db, sale.Sales{
		ProductID: 1, BatchID: &b,
		Productor: "Apple", Name: "iPhone 15",
		Price: decimal.NewFromInt(100), Sales: 10, Month: "2024-01",
	})
	g := Grain{ProductID: 1, Productor: "Apple", Name: "iPhone 15", Month: "2024-01"}
	if _, err := repo.RefreshGrains([]Grain{g}); err != nil {
		t.Fatalf("首次刷新失败: %v", err)
	}

	if err := db.Where("batch_id = ?", b).Delete(&sale.Sales{}).Error; err != nil {
		t.Fatalf("删除明细失败: %v", err)
	}
	if _, err := repo.RefreshGrains([]Grain{g}); err != nil {
		t.Fatalf("删除后刷新失败: %v", err)
	}

	var count int64
	db.Model(&SalesSummary{}).
		Where("product_id = ? AND productor = ? AND name = ? AND month = ?",
			g.ProductID, g.Productor, g.Name, g.Month).
		Count(&count)
	if count != 0 {
		t.Fatalf("期望汇总行被删除，实际还有 %d 行", count)
	}
}

// 同一 grain 由两个批次贡献，删除其中一个后必须按剩余批次重算。
func TestRefreshGrains_DeleteOneBatchRecomputesRemaining(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	b1, b2 := uint(1), uint(2)
	seedSales(t, db,
		sale.Sales{ProductID: 1, BatchID: &b1, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(100), Sales: 2, Month: "2024-01"},
		sale.Sales{ProductID: 1, BatchID: &b2, Productor: "Apple", Name: "iPhone 15", Price: decimal.NewFromInt(50), Sales: 3, Month: "2024-01"},
	)
	g := Grain{ProductID: 1, Productor: "Apple", Name: "iPhone 15", Month: "2024-01"}
	if _, err := repo.RefreshGrains([]Grain{g}); err != nil {
		t.Fatalf("首次刷新失败: %v", err)
	}

	if err := db.Where("batch_id = ?", b1).Delete(&sale.Sales{}).Error; err != nil {
		t.Fatalf("删除明细失败: %v", err)
	}
	if _, err := repo.RefreshGrains([]Grain{g}); err != nil {
		t.Fatalf("删除后刷新失败: %v", err)
	}

	var got SalesSummary
	if err := db.Where("product_id = ? AND month = ?", 1, "2024-01").First(&got).Error; err != nil {
		t.Fatalf("读取汇总行失败: %v", err)
	}
	if got.TotalSales != 3 || got.TotalAmount.Cmp(decimal.NewFromInt(150)) != 0 {
		t.Errorf("期望销量 3、金额 150，实际 %d / %v", got.TotalSales, got.TotalAmount)
	}
	if got.RecordCount != 1 || got.BatchCount != 1 {
		t.Errorf("期望记录数 1、批次数 1，实际 %d / %d", got.RecordCount, got.BatchCount)
	}
}

// worker 必须消费 outbox 任务，刷新汇总并把任务标记为 done。
func TestWorker_ProcessesEnqueuedTask(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	b := uint(1)
	seedSales(t, db, sale.Sales{
		ProductID: 1, BatchID: &b,
		Productor: "Apple", Name: "iPhone 15",
		Price: decimal.NewFromInt(100), Sales: 10, Month: "2024-01",
	})
	g := Grain{ProductID: 1, Productor: "Apple", Name: "iPhone 15", Month: "2024-01"}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return EnqueueRefreshTx(tx, []Grain{g})
	}); err != nil {
		t.Fatalf("写入任务失败: %v", err)
	}

	w := NewWorker(repo, time.Second)
	processed, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("处理任务失败: %v", err)
	}
	if !processed {
		t.Fatal("期望处理一条任务")
	}

	var task RefreshTask
	if err := db.First(&task).Error; err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if task.Status != TaskDone {
		t.Errorf("任务状态期望 %s，实际 %s", TaskDone, task.Status)
	}

	var got SalesSummary
	if err := db.Where("product_id = ? AND month = ?", 1, "2024-01").First(&got).Error; err != nil {
		t.Fatalf("读取汇总行失败: %v", err)
	}
	if got.TotalSales != 10 {
		t.Errorf("total_sales 期望 10，实际 %d", got.TotalSales)
	}
}

// 刷新失败时任务必须回到 pending，并记录 attempts 和错误。
func TestWorker_RecordsFailureAndRetries(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)

	task := RefreshTask{
		Grains:   "not-json",
		Status:   TaskPending,
		RunAfter: time.Now().Add(-time.Second),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("写入任务失败: %v", err)
	}

	w := NewWorker(repo, time.Second)
	processed, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("处理任务返回错误: %v", err)
	}
	if !processed {
		t.Fatal("期望处理一条任务")
	}

	var got RefreshTask
	if err := db.First(&got, task.ID).Error; err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if got.Status != TaskPending {
		t.Errorf("失败后状态期望 %s，实际 %s", TaskPending, got.Status)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts 期望 1，实际 %d", got.Attempts)
	}
	if !strings.Contains(got.LastError, "解析任务") {
		t.Errorf("last_error 未记录解析错误: %q", got.LastError)
	}
}
