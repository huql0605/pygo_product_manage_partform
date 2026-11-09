package summary

import "gorm.io/gorm"

// refreshLockKey 串行化所有汇总刷新，避免同一 grain 的并发 delete/insert 互相覆盖。
const refreshLockKey int64 = 0x6477735f72656672

// Grain 是 dws_sales 的聚合粒度。
type Grain struct {
	ProductID uint   `json:"product_id" gorm:"column:product_id"`
	Productor string `json:"productor" gorm:"column:productor"`
	Name      string `json:"name" gorm:"column:name"`
	Month     string `json:"month" gorm:"column:month"`
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GrainsByBatchTx 在事务内读取某批次当前覆盖的 grain。
// transform 在替换明细前后各调用一次，才能覆盖 grain 发生变化的旧行。
func (r Repository) GrainsByBatchTx(tx *gorm.DB, batchID uint) ([]Grain, error) {
	var out []Grain
	err := tx.Raw(`
		SELECT DISTINCT product_id, productor, name, month
		FROM sales
		WHERE batch_id = ?
	`, batchID).Scan(&out).Error
	return out, err
}

func (r Repository) GrainsByBatch(batchID uint) ([]Grain, error) {
	return r.GrainsByBatchTx(r.db, batchID)
}

// Refresh 按批次当前明细重算。没有明细时是空操作，删除批次不要走这里。
func (r Repository) Refresh(batchID uint) (int64, error) {
	grains, err := r.GrainsByBatch(batchID)
	if err != nil {
		return 0, err
	}
	return r.RefreshGrains(grains)
}

// RefreshGrains 只重算指定 grain。明细已不存在的 grain 会删除对应的汇总行。
func (r Repository) RefreshGrains(grains []Grain) (int64, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		return r.RefreshGrainsTx(tx, grains)
	})
	if err != nil {
		return 0, err
	}
	return r.count()
}

// RefreshGrainsTx 在调用方事务内重算指定 grain。
// worker 用它把「取任务 + 刷新 + 标记完成」放在同一个事务里。
func (r Repository) RefreshGrainsTx(tx *gorm.DB, grains []Grain) error {
	grains = uniqueGrains(grains)

	if err := tx.Exec(`SELECT pg_advisory_xact_lock(?)`, refreshLockKey).Error; err != nil {
		return err
	}
	if len(grains) == 0 {
		return nil
	}
	if err := tx.Exec(`
		CREATE TEMP TABLE affected_grain (
			product_id bigint NOT NULL,
			productor text NOT NULL,
			name text NOT NULL,
			month varchar(7) NOT NULL
		) ON COMMIT DROP;
	`).Error; err != nil {
		return err
	}
	if err := tx.Table("affected_grain").CreateInBatches(grains, 500).Error; err != nil {
		return err
	}
	if err := tx.Exec(`
		DELETE FROM dws_sales d
		USING affected_grain a
		WHERE d.product_id = a.product_id
		AND d.productor = a.productor
		AND d.name = a.name
		AND d.month = a.month;
	`).Error; err != nil {
		return err
	}
	return tx.Exec(`
		INSERT INTO dws_sales (
			product_id , productor , name , month , total_sales , total_amount , price_sum , record_count , min_price , max_price  , batch_count , updated_at
		)
		SELECT
			s.product_id,
			s.productor,
			s.name,
			s.month,
			COALESCE(SUM(s.sales),0),
			COALESCE(SUM(s.sales * s.price),0),
			COALESCE(SUM(s.price),0),
			COUNT(*),
			COALESCE(MIN(s.price),0),
			COALESCE(MAX(s.price),0),
			COUNT(DISTINCT s.batch_id),
			now()
		FROM sales s
		JOIN affected_grain a
		ON s.product_id = a.product_id
		AND s.productor = a.productor
		AND s.name = a.name
		AND s.month = a.month
		GROUP BY s.product_id, s.productor, s.name, s.month;
	`).Error
}

// RefreshAll 全量重建，作为手工兜底。DELETE 不取 ACCESS EXCLUSIVE 锁，读请求不会被完全阻塞。
func (r Repository) RefreshAll() (int64, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(?)`, refreshLockKey).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM dws_sales").Error; err != nil {
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
	return r.count()
}

func (r Repository) count() (int64, error) {
	var count int64
	if err := r.db.Model(&SalesSummary{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func uniqueGrains(in []Grain) []Grain {
	seen := make(map[Grain]struct{}, len(in))
	out := make([]Grain, 0, len(in))
	for _, g := range in {
		if _, ok := seen[g]; ok {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	return out
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
