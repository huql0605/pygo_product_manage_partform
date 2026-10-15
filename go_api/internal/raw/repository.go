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
