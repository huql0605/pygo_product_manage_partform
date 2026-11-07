package raw

import (
	"errors"

	"github.com/huql0605/go_api/internal/summary"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

func (r *Repository) CreateManualBatchTx(tx *gorm.DB, category string, rowCount int) (uint, error) {
	batch := &ImportBatch{
		Source:   "manual",
		Category: category,
		Status:   StatusMapped,
		RowCount: rowCount,
	}
	if err := tx.Create(batch).Error; err != nil {
		return 0, err
	}
	return batch.ID, nil
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
// 删除前先记录该批次覆盖的 grain，供 summary 在事务提交后重算；
// 删除后再查 sales 已经查不到这些 grain。
func (r *Repository) PurgeBatch(id uint) (int64, []summary.Grain, error) {
	var deletedSales int64
	var grains []summary.Grain

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var batch ImportBatch
		res := tx.Exec("SET LOCAL lock_timeout = '5s';")
		if res.Error != nil {
			return res.Error
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&batch, id).Error; err != nil {
			return err
		}

		if err := tx.Raw(`
			SELECT DISTINCT product_id, productor, name, month
			FROM sales
			WHERE batch_id = ?
		`, id).Scan(&grains).Error; err != nil {
			return err
		}

		res = tx.Exec("DELETE FROM sales WHERE batch_id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		deletedSales = res.RowsAffected

		if err := tx.Where("batch_id = ?", id).Delete(&RawRecord{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ImportBatch{}, id).Error
	})
	return deletedSales, grains, err
}

func (r *Repository) CreateBatchWithFile(batch *ImportBatch, file *RawFile, records []RawRecord) (bool, error) {
	dupilcated := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var stored RawFile
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sha256"}},
			DoNothing: true,
		}).Create(file)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if err := tx.Where("sha256 = ?", file.SHA256).First(&stored).Error; err != nil {
				return err
			}

		} else {
			stored = *file
		}
		batch.RawFileID = &stored.ID
		batch.FileHash = stored.SHA256

		var existing ImportBatch
		err := tx.Where("category = ? AND raw_file_id = ?", batch.Category, batch.RawFileID).First(&existing).Error
		if err == nil {
			*batch = existing
			dupilcated = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		res = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(batch)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if err := tx.Where("category = ? AND raw_file_id = ?", batch.Category, batch.RawFileID).First(&existing).Error; err != nil {
				return err
			}
			*batch = existing
			dupilcated = true
			return nil
		}

		for i := range records {
			records[i].BatchID = batch.ID
		}
		if len(records) == 0 {
			return nil
		}
		return tx.CreateInBatches(records, 500).Error
	})
	return dupilcated, err
}

func (r *Repository) GetBatchForUpdateTx(tx *gorm.DB, id uint) (*ImportBatch, error) {
	if err := tx.Exec("SET LOCAL lock_timeout = '5s'").Error; err != nil {
		return nil, err
	}

	var b ImportBatch
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&b, id).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *Repository) ListRecordsTx(tx *gorm.DB, batchID uint) ([]RawRecord, error) {
	var out []RawRecord
	err := tx.Where("batch_id = ?", batchID).Order("row_index").Find(&out).Error
	return out, err
}

func (r *Repository) UpdateStatusTx(tx *gorm.DB, batchId uint, status string) error {
	return tx.Model(&ImportBatch{}).Where("id = ?", batchId).Update("status", status).Error
}
