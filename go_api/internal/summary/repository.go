package summary

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r Repository) RefreshAll() (int64, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("TRUNCATE TABLE dws_sales").Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO dws_sales (
				product_id , productor , name , month , total_sales , total_amount , price_sum , record_count , min_price , max_price  , batch_count , updated_at
			)
			SELECT 
				product_id,
				productor,
				name,
				month,
				COALESCE(SUM(sales),0),
				COALESCE(SUM(sales * price),0),
				COALESCE(SUM(price),0),
				COUNT(*),
				COALESCE(MIN(price),0),
				COALESCE(MAX(price),0),
				COUNT(DISTINCT batch_id),
				now()
			FROM sales
			GROUP BY product_id,productor,name,month
		`).Error
	})
	if err != nil {
		return 0, err
	}
	var count int64
	if err := r.db.Model(&SalesSummary{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil

}

type Diff struct {
	ProductID  uint   `json:"product_id"`
	Productor  string `json:"productor"`
	Name       string `json:"name"`
	Month      string `json:"month"`
	TotalSales int64  `json:"total_sales"`
	Side       string `json:"side"` // detail_only / summary_only
}

// Verify 对账：双向 EXCEPT，找出明细与汇总不一致的 grain。
func (r *Repository) Verify() ([]Diff, error) {
	var diffs []Diff
	err := r.db.Raw(`
        WITH detail AS (
            SELECT product_id, productor, name, month, SUM(sales) AS total_sales
            FROM sales
            GROUP BY product_id, productor, name, month
        ),
        dws AS (
            SELECT product_id, productor, name, month, SUM(total_sales) AS total_sales
            FROM dws_sales
            GROUP BY product_id, productor, name, month
        )
        SELECT product_id, productor, name, month, total_sales, 'detail_only' AS side
        FROM (SELECT * FROM detail EXCEPT SELECT * FROM dws) d
        UNION ALL
        SELECT product_id, productor, name, month, total_sales, 'summary_only' AS side
        FROM (SELECT * FROM dws EXCEPT SELECT * FROM detail) s
    `).Scan(&diffs).Error
	return diffs, err
}
