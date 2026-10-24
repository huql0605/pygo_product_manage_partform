package raw

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateBatch(batch *ImportBatch, records []RawRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		for i := range records {
			records[i].BatchID = batch.ID
		}
		return tx.CreateInBatches(records, 500).Error
	})

}

func (r *Repository) GetBatch(id uint) (*ImportBatch, error) {
	var b ImportBatch
	if err := r.db.First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *Repository) ListColumns(batchID uint) ([]ColumnInfo, error) {
	var out []ColumnInfo
	err := r.db.Raw(`
		SELECT c.key AS column_name , MIN(r.row_data ->> c.key) AS 	sample_value
		FROM raw_records r
		CROSS JOIN LATERAL jsonb_object_keys(r.row_data) AS c(key)
		WHERE r.batch_id =?
		GROUP BY c.key
		ORDER BY c.key`, batchID).Scan(&out).Error
	return out, err
}

func (r *Repository) ListRecords(batchID uint) ([]RawRecord, error) {
	var out []RawRecord
	err := r.db.Where("batch_id = ? ", batchID).Order("row_index").Find(&out).Error
	return out, err
}

func (r *Repository) UpdateStatus(BatchID uint, status string) error {
	return r.db.Model(&ImportBatch{}).Where("id = ? ", BatchID).Update("status", status).Error

}

// CountRecords 统计某批次已入库的原始行数。
func (r *Repository) CountRecords(batchID uint) (int64, error) {
	var count int64
	err := r.db.Model(&RawRecord{}).Where("batch_id = ?", batchID).Count(&count).Error
	return count, err
}

// ListBatches 按品类 / 状态筛选批次，返回当前页与总数。
func (r *Repository) ListBatches(category, status string, offset, limit int) ([]ImportBatch, int64, error) {
	// 每次重新构造查询，避免 Count 之后复用同一条语句带来的状态污染
	base := func() *gorm.DB {
		q := r.db.Model(&ImportBatch{})
		if category != "" {
			q = q.Where("category = ?", category)
		}
		if status != "" {
			q = q.Where("status = ?", status)
		}
		return q
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	out := make([]ImportBatch, 0, limit)
	err := base().Order("id DESC").Offset(offset).Limit(limit).Find(&out).Error
	return out, total, err
}

// PurgeBatch 在一个事务里删除批次及其全部衍生数据：
// sales 明细行（batch_id 指向它）、raw_records、import_batches。
//
// 这里直接写 sales 表名，是因为跨层删除必须落在同一个事务里；
// 若拆成两个 repository 调用，中途失败会留下"明细已删、原始数据还在"的脏状态。
func (r *Repository) PurgeBatch(id uint) (int64, error) {
	var deletedSales int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Exec("DELETE FROM sales WHERE batch_id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		deletedSales = res.RowsAffected

		if err := tx.Where("batch_id = ?", id).Delete(&RawRecord{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ImportBatch{}, id).Error
	})
	return deletedSales, err
}
