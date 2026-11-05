package sale

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateMany(rows []Sales) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.CreateInBatches(rows, 500).Error
	})
}

func (r *Repository) CreateManyTx(tx *gorm.DB, rows []Sales) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.CreateInBatches(rows, 500).Error
}

func (r *Repository) ReplaceBatchTx(tx *gorm.DB, batchId uint, rows []Sales) error {
	if err := tx.Where("batch_id = ? ", batchId).Delete(&Sales{}).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.CreateInBatches(rows, 500).Error
}

// ReplaceBatch 在一个事务里「先删该批次、再插入」，保证幂等。
func (r *Repository) ReplaceBatch(batchID uint, rows []Sales) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.ReplaceBatchTx(tx, batchID, rows)
	})
}
